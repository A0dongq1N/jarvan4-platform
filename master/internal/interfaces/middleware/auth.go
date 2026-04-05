package middleware

import (
	"context"
	"net/http"
	"strings"
)

// contextKey 避免与其他包的 key 冲突
type contextKey int

const (
	keyUserID   contextKey = iota
	keyUsername contextKey = iota
	keyRole     contextKey = iota
)

// TokenParser JWT 令牌解析接口（由 app/auth.Service 实现）
type TokenParser interface {
	Parse(token string) (userID, username string, role int8, err error)
}

// Auth JWT 鉴权中间件（标准 net/http）
func Auth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 登录/短信接口跳过鉴权
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeUnauthorized(w, "invalid Authorization format")
				return
			}
			userID, username, role, err := parser.Parse(parts[1])
			if err != nil {
				writeUnauthorized(w, "invalid token")
				return
			}

			r = WithUser(r, userID, username, role)
			next.ServeHTTP(w, r)
		})
	}
}

// WithUser 将用户信息注入 request context
func WithUser(r *http.Request, userID, username string, role int8) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, keyUserID, userID)
	ctx = context.WithValue(ctx, keyUsername, username)
	ctx = context.WithValue(ctx, keyRole, role)
	return r.WithContext(ctx)
}

// CurrentUserID 从 context 中取用户 ID
func CurrentUserID(r *http.Request) string {
	v, _ := r.Context().Value(keyUserID).(string)
	return v
}

// CurrentUsername 从 context 中取用户名
func CurrentUsername(r *http.Request) string {
	v, _ := r.Context().Value(keyUsername).(string)
	return v
}

// CurrentRole 从 context 中取角色
func CurrentRole(r *http.Request) int8 {
	v, _ := r.Context().Value(keyRole).(int8)
	return v
}

func isPublicPath(path string) bool {
	publicPaths := []string{
		"/api/auth/login",
		"/api/auth/login/sms",
		"/api/auth/sms/send",
		"/api/internal/scripts/publish", // CI 回调，无需 token
	}
	for _, p := range publicPaths {
		if path == p {
			return true
		}
	}
	// Worker 心跳：/api/internal/workers/{worker_id}/heartbeat
	if strings.HasPrefix(path, "/api/internal/workers/") && strings.HasSuffix(path, "/heartbeat") {
		return true
	}
	return false
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"code":401,"message":"` + msg + `"}`))
}
