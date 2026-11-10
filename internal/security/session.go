package security

import (
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
)

// ContextSessionKey gin 上下文中保存当前会话的键。
const ContextSessionKey = "session"

// ContextUserScopeKey gin 上下文中保存当前用户资源范围的键。
const ContextUserScopeKey = "user_scope"

// anonymousUserID 匿名模式下的固定访客标识，用于资源归属字段的默认值。
const anonymousUserID = "anonymous"

// Session 当前访问者会话。
//
// 本版本为匿名单用户模式：不登录、不区分用户，任何请求都持有全部权限与全局资源范围。
// 保留该结构是为了让调用方无需判断登录态，也便于日后重新引入账户体系。
type Session struct {
	Token            string
	UserID           string
	Username         string
	DisplayName      string
	Roles            []string
	Permissions      map[string]bool
	PermissionScopes map[string]string
	Scope            string
}

// anonymousSession 匿名模式下唯一的会话实例。
var anonymousSession = &Session{
	UserID:           anonymousUserID,
	Username:         anonymousUserID,
	DisplayName:      "匿名用户",
	Permissions:      map[string]bool{},
	PermissionScopes: map[string]string{},
	Scope:            database.RBACScopeAll,
}

// AnonymousSessionMiddleware 为每个请求挂载匿名的全权限会话。
func AnonymousSessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextSessionKey, anonymousSession)
		c.Set(ContextUserScopeKey, database.RBACScopeAll)
		c.Next()
	}
}

// CurrentSession 返回当前请求的会话。匿名模式下恒有会话，不存在未登录状态。
func CurrentSession(c *gin.Context) (*Session, bool) {
	if value, ok := c.Get(ContextSessionKey); ok {
		if session, ok := value.(*Session); ok && session != nil {
			return session, true
		}
	}
	return anonymousSession, true
}

// SessionHasPermission 判断当前请求是否具备指定权限。匿名模式下恒为 true。
func SessionHasPermission(c *gin.Context, permission string) bool {
	_, ok := CurrentSession(c)
	return ok
}

// ScopeFor 返回指定权限对应的资源范围。匿名模式下恒为全局范围。
func (s *Session) ScopeFor(permission string) string {
	if s != nil {
		if scope, ok := s.PermissionScopes[permission]; ok && scope != "" {
			return scope
		}
		if s.Scope != "" {
			return s.Scope
		}
	}
	return database.RBACScopeAll
}
