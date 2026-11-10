package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestLegacyProjectForeignKeyIsRepaired 覆盖旧库升级：projects 表被删除后，
// conversations 上残留的外键会让插入直接失败（no such table: main.projects）。
func TestLegacyProjectForeignKeyIsRepaired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite3", sqliteOpenDSN(path))
	if err != nil {
		t.Fatalf("打开旧库失败: %v", err)
	}
	legacy := []string{
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE conversations (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			role_name TEXT NOT NULL DEFAULT '',
			agent_mode TEXT NOT NULL DEFAULT 'eino_single',
			project_id TEXT REFERENCES projects(id) ON DELETE SET NULL
		)`,
		`INSERT INTO projects (id, name) VALUES ('p1', 'legacy')`,
		`INSERT INTO conversations (id, title, created_at, updated_at, role_name, agent_mode, project_id)
		 VALUES ('c1', 'kept', '2026-01-01 10:00:00', '2026-01-01 10:00:00', 'default', 'eino_single', 'p1')`,
	}
	for _, stmt := range legacy {
		if _, err := raw.Exec(stmt); err != nil {
			raw.Close()
			t.Fatalf("准备旧库失败: %v", err)
		}
	}
	raw.Close()

	db, err := NewDB(path, zap.NewNop())
	if err != nil {
		t.Fatalf("升级旧库失败: %v", err)
	}
	defer db.Close()

	var projectsTable int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='projects'`).Scan(&projectsTable); err != nil {
		t.Fatalf("查询 projects 表失败: %v", err)
	}
	if projectsTable != 0 {
		t.Fatalf("projects 表应已删除")
	}

	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='conversations'`).Scan(&ddl); err != nil {
		t.Fatalf("读取 conversations 结构失败: %v", err)
	}
	if strings.Contains(strings.ToUpper(ddl), "REFERENCES PROJECTS") {
		t.Fatalf("conversations 仍残留指向 projects 的外键:\n%s", ddl)
	}

	// 旧数据保留，且新建对话不再触发外键错误。
	var title string
	if err := db.QueryRow(`SELECT title FROM conversations WHERE id='c1'`).Scan(&title); err != nil || title != "kept" {
		t.Fatalf("旧数据未保留: %q %v", title, err)
	}
	conv, err := db.CreateConversation("after-upgrade", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("升级后创建对话失败: %v", err)
	}
	if conv.ID == "" {
		t.Fatalf("创建对话返回空 ID")
	}
}
