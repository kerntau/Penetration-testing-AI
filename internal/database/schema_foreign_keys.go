package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// repairRemovedForeignKeys 重建仍带有「指向已下线表」外键的旧表。
//
// SQLite 不支持删除外键约束，只能按当前 schema 重建表并搬迁数据。升级到精简版后
// projects 等表会被删除，若 conversations.model_token_usage 仍保留 REFERENCES projects(id)，
// 任何插入都会因为找不到父表而失败（no such table: main.projects）。
func (db *DB) repairRemovedForeignKeys() error {
	reference, err := openSchemaReference(applicationSchema)
	if err != nil {
		return err
	}
	defer reference.Close()

	removed := make(map[string]struct{}, len(removedFeatureTables))
	for _, name := range removedFeatureTables {
		removed[name] = struct{}{}
	}

	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL`)
	if err != nil {
		return err
	}
	type legacyTable struct{ name, ddl string }
	var pending []legacyTable
	for rows.Next() {
		var name, ddl string
		if scanErr := rows.Scan(&name, &ddl); scanErr != nil {
			rows.Close()
			return scanErr
		}
		if tableReferencesRemoved(ddl, removed) {
			pending = append(pending, legacyTable{name: name, ddl: ddl})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 重建期间关闭外键检查，并阻止 RENAME 改写其它表里的引用。
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	if _, err := conn.ExecContext(ctx, "PRAGMA legacy_alter_table=ON"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "PRAGMA legacy_alter_table=OFF")

	for _, table := range pending {
		if err := rebuildTableFromReference(ctx, conn, reference, table.name); err != nil {
			return fmt.Errorf("重建旧表 %s 失败: %w", table.name, err)
		}
		db.logger.Info("已重建带过期外键的旧表", zap.String("table", table.name))
	}
	return nil
}

// tableReferencesRemoved 判断建表语句里是否存在指向已下线表的外键。
func tableReferencesRemoved(ddl string, removed map[string]struct{}) bool {
	upper := strings.ToUpper(ddl)
	if !strings.Contains(upper, "REFERENCES") {
		return false
	}
	rest := ddl
	for {
		idx := strings.Index(strings.ToUpper(rest), "REFERENCES")
		if idx < 0 {
			return false
		}
		rest = rest[idx+len("REFERENCES"):]
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" {
			return false
		}
		// 表标识符可能紧跟括号（REFERENCES projects(id)），也可能被引号包裹。
		end := len(rest)
		if i := strings.IndexAny(rest, " \t\r\n("); i >= 0 {
			end = i
		}
		name := strings.Trim(rest[:end], `"';`)
		name = strings.Trim(name, "[]`")
		if _, ok := removed[strings.ToLower(name)]; ok {
			return true
		}
	}
}

// rebuildTableFromReference 按参考 schema 重建单张表，并搬迁两表共有的列。
func rebuildTableFromReference(ctx context.Context, conn *sql.Conn, reference *sql.DB, table string) error {
	var refDDL string
	err := reference.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=? COLLATE NOCASE`, table).Scan(&refDDL)
	if err == sql.ErrNoRows {
		// 参考 schema 里已没有这张表：直接丢弃，避免残留过期结构。
		_, dropErr := conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoteSchemaName(table))
		return dropErr
	}
	if err != nil {
		return err
	}

	legacy := table + "__legacy"
	quoted := quoteSchemaName(table)
	quotedLegacy := quoteSchemaName(legacy)

	if _, err := conn.ExecContext(ctx, "ALTER TABLE "+quoted+" RENAME TO "+quotedLegacy); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, refDDL); err != nil {
		return err
	}

	newCols, err := connColumns(ctx, conn, table)
	if err != nil {
		return err
	}
	oldCols, err := connColumns(ctx, conn, legacy)
	if err != nil {
		return err
	}
	oldSet := make(map[string]struct{}, len(oldCols))
	for _, c := range oldCols {
		oldSet[c] = struct{}{}
	}
	common := make([]string, 0, len(newCols))
	for _, c := range newCols {
		if _, ok := oldSet[c]; ok {
			common = append(common, c)
		}
	}
	if len(common) > 0 {
		quotedCols := make([]string, 0, len(common))
		for _, c := range common {
			quotedCols = append(quotedCols, quoteSchemaName(c))
		}
		copySQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
			quoted, strings.Join(quotedCols, ", "), strings.Join(quotedCols, ", "), quotedLegacy)
		if _, err := conn.ExecContext(ctx, copySQL); err != nil {
			return err
		}
	}
	if _, err := conn.ExecContext(ctx, "DROP TABLE "+quotedLegacy); err != nil {
		return err
	}

	// 旧索引已随 legacy 表删除，按参考 schema 重建。
	indexRows, err := reference.Query(`SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? COLLATE NOCASE AND sql IS NOT NULL`, table)
	if err != nil {
		return err
	}
	var indexSQL []string
	for indexRows.Next() {
		var stmt string
		if scanErr := indexRows.Scan(&stmt); scanErr != nil {
			indexRows.Close()
			return scanErr
		}
		indexSQL = append(indexSQL, stmt)
	}
	indexRows.Close()
	if err := indexRows.Err(); err != nil {
		return err
	}
	for _, stmt := range indexSQL {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("重建索引失败: %w", err)
		}
	}
	return nil
}

// connColumns 返回指定表的列名（按定义顺序，列名统一小写）。
func connColumns(ctx context.Context, conn *sql.Conn, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA table_info("+quoteSchemaName(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, strings.ToLower(name))
	}
	return cols, rows.Err()
}
