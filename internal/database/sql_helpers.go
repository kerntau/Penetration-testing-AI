package database

// nullIfEmpty 将空字符串转为 SQL NULL，便于写入可空列。
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// boolToInt 将布尔值转换为 SQLite 使用的 0/1。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
