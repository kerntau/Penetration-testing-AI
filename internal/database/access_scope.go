package database

// RBACScopeAll 表示不受资源归属限制的全局可见范围。
// 当前为匿名单用户模式，所有查询恒使用该范围。
const RBACScopeAll = "all"

// UserCanAccessResource 判断用户是否可访问指定资源。
// 匿名单用户模式下不存在用户隔离，恒为可访问。
func (db *DB) UserCanAccessResource(userID, scope, resourceType, resourceID string) bool {
	return true
}
