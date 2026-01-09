package middleware

import (
	"net/http"
	"strings"

	"github.com/lemonc7/zest"
)

// JWTer 定义了 JWT 解析和验证的接口
// 任何实现了 Parse 方法的类型都可以用于 JWT 认证中间件
type JWTer interface {
	Parse(tokenString string) (map[string]any, error)
}

// JWT 返回 JWT 认证中间件
// 只支持 "Authorization: Bearer <token>" 格式
// skipper 可选参数：返回 true 时跳过认证
func JWT(j JWTer, skipper ...func(*zest.Context) bool) zest.MiddlewareFunc {
	skip := func(c *zest.Context) bool { return false }
	if len(skipper) > 0 && skipper[0] != nil {
		skip = skipper[0]
	}

	return func(next zest.HandlerFunc) zest.HandlerFunc {
		return func(c *zest.Context) error {
			// 跳过认证
			if skip(c) {
				return next(c)
			}

			authHeader := c.Request.Header.Get("Authorization")
			if authHeader == "" {
				return zest.NewHTTPError(http.StatusUnauthorized, "missing token")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				return zest.NewHTTPError(http.StatusUnauthorized, "invalid token format")
			}

			tokenString := parts[1]
			claims, err := j.Parse(tokenString)
			if err != nil {
				return zest.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			// 将 claims 存入 context
			for k, v := range claims {
				c.Set(k, v)
			}

			return next(c)
		}
	}
}
