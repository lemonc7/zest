package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lemonc7/zest"
)

// CORSConfig CORS 配置
type CORSConfig struct {
	// 允许的域名，["*"] 表示所有
	AllowOrigins []string
	// AllowOriginFunc 自定义判断 origin 是否合法的函数
	// 如果设置了此函数，AllowOrigins 将被忽略
	AllowOriginFunc func(origin string) bool
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
func CORS(config ...CORSConfig) zest.MiddlewareFunc {
	// 1. 初始化配置，确保都有默认值
	cfg := DefaultCORSConfig
	if len(config) > 0 {
		userCfg := config[0]

		// 只有当用户显式设置了某个字段（非零值/非空）时，才覆盖默认值
		// 注意：如果用户真的想设置为空列表（禁用所有），这里的逻辑会 fallback 到默认值
		// 为了支持"显式禁用"，通常需要更复杂的逻辑（如指针）。
		// 但对于 CORS，通常不需要"允许的方法为空"，所以这里简化处理：只要用户填了就用用户的，没填就用默认的。

		if len(userCfg.AllowOrigins) > 0 {
			cfg.AllowOrigins = userCfg.AllowOrigins
		}
		if userCfg.AllowOriginFunc != nil {
			cfg.AllowOriginFunc = userCfg.AllowOriginFunc
		}
		if len(userCfg.AllowMethods) > 0 {
			cfg.AllowMethods = userCfg.AllowMethods
		}
		if len(userCfg.AllowHeaders) > 0 {
			cfg.AllowHeaders = userCfg.AllowHeaders
		}
		if len(userCfg.ExposeHeaders) > 0 {
			cfg.ExposeHeaders = userCfg.ExposeHeaders
		}
		// bool 和 time.Duration 直接赋值（因为 False 和 0 也是有效值）
		// 如果用户其实想留空用默认，这里可能会有问题，但在 Go 这种 Options 模式下，通常假设用户构建 Config 时知道自己在做什么
		// 这里还是保留用户传入的值
		cfg.AllowCredentials = userCfg.AllowCredentials
		if userCfg.MaxAge > 0 {
			cfg.MaxAge = userCfg.MaxAge
		}
	}

	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := strconv.FormatInt(int64(cfg.MaxAge.Seconds()), 10)

	return func(next zest.HandlerFunc) zest.HandlerFunc {
		return func(c *zest.Context) error {
			origin := c.Request.Header.Get("Origin")
			// 即使没有 Origin，也可以是同源请求，CORS 规范通常只在跨域时生效
			// 但很多客户端库会发 Origin，保守起见如果没 Origin 直接放行
			if origin == "" {
				return next(c)
			}

			// 检查 origin 是否被允许
			allowOrigin := ""

			if cfg.AllowOriginFunc != nil {
				if cfg.AllowOriginFunc(origin) {
					allowOrigin = origin
				}
			} else {
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
			}

			if allowOrigin == "" {
				// Origin 不被允许，通常做法是：
				// 1. 返回 403 (严格模式)
				// 2. 忽略 CORS 头，当作普通请求处理，由浏览器拦截响应 (宽松模式)
				// 这里采用宽松模式，不设 Header，浏览器一看没 Header 自己就报错了
				return next(c)
			}

			// 设置 CORS 响应头
			c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
			// Vary Header 非常重要，告诉缓存服务器响应内容取决于 Origin
			if len(cfg.AllowOrigins) > 1 || cfg.AllowOriginFunc != nil {
				c.SetHeader("Vary", "Origin")
			}

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
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}
