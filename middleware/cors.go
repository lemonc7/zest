package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lemonc7/engx"
)

// CORSConfig CORS 配置
type CORSConfig struct {
	// 允许的域名，["*"] 表示所有
	AllowOrigins []string
	// 允许的 HTTP 方法
	AllowMethods []string
	// 允许的请求头
	AllowHeaders []string
	// 暴露给客户端的响应头
	ExposeHeaders []string
	// 是否允许携带凭证 (cookies, authorization headers)
	AllowCredentials bool
	// 预检请求缓存时间（秒）
	MaxAge time.Duration
}

// DefaultCORSConfig 默认配置
var DefaultCORSConfig = CORSConfig{
	AllowOrigins: []string{"*"},
	AllowMethods: []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	},
	AllowHeaders: []string{
		"Origin",
		"Content-Type",
		"Authorization",
	},
	MaxAge: 24 * time.Hour,
}

// CORS 返回 CORS 中间件
func CORS(config ...CORSConfig) engx.MiddlewareFunc {
	cfg := DefaultCORSConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := strconv.FormatInt(int64(cfg.MaxAge.Seconds()), 10)

	return func(next engx.HandlerFunc) engx.HandlerFunc {
		return func(c *engx.Context) error {
			origin := c.Request.Header.Get("Origin")
			if origin == "" {
				return next(c)
			}

			// 检查 origin 是否被允许
			allowOrigin := ""
			for _, o := range cfg.AllowOrigins {
				if o == "*" || o == origin {
					if cfg.AllowCredentials && o == "*" {
						allowOrigin = origin
					} else {
						allowOrigin = o
					}
					break
				}
			}

			if allowOrigin == "" {
				return next(c)
			}

			// 设置 CORS 响应头
			c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
			c.SetHeader("Vary","Origin") // 防止CDN缓存错乱
			if cfg.AllowCredentials {
				c.SetHeader("Access-Control-Allow-Credentials", "true")
			}
			if expose != "" {
				c.SetHeader("Access-Control-Expose-Headers", expose)
			}

			// 处理预检请求 (OPTIONS)
			if c.Request.Method == http.MethodOptions {
				c.SetHeader("Access-Control-Allow-Methods", methods)
				c.SetHeader("Access-Control-Allow-Headers", headers)
				if cfg.MaxAge > 0 {
					c.SetHeader("Access-Control-Max-Age", maxAge)
				}
				c.SetStatus(http.StatusNoContent)
				return nil
			}

			return next(c)
		}
	}
}
