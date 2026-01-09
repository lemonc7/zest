package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/lemonc7/zest"
)

// RequestIDConfig RequestID 中间件配置
type RequestIDConfig struct {
	// Header 响应头中的 RequestID 字段名
	Header string
	// Generator 生成 RequestID 的函数
	Generator func() string
}

// DefaultRequestIDConfig 默认配置
var DefaultRequestIDConfig = RequestIDConfig{
	Header: "X-Request-ID",
	Generator: func() string {
		var id [16]byte
		_, _ = rand.Read(id[:])
		return hex.EncodeToString(id[:])
	},
}

// RequestID 返回一个生成唯一请求 ID 的中间件
func RequestID(config ...RequestIDConfig) zest.MiddlewareFunc {
	cfg := DefaultRequestIDConfig
	if len(config) > 0 {
		cfg = config[0]
		if cfg.Header == "" {
			cfg.Header = DefaultRequestIDConfig.Header
		}
		if cfg.Generator == nil {
			cfg.Generator = DefaultRequestIDConfig.Generator
		}
	}

	return func(next zest.HandlerFunc) zest.HandlerFunc {
		return func(c *zest.Context) error {
			// 1. 获取或生成 RequestID
			rid := c.Request.Header.Get(cfg.Header)
			if rid == "" {
				rid = cfg.Generator()
			}

			// 2. 注入到响应头与上下文，方便跨函数传递
			c.SetHeader(cfg.Header, rid)
			ctx := context.WithValue(c.Context(), "requestID", rid)
			c.Request = c.Request.WithContext(ctx)

			// 3. 注入到 Context 存储中，方便后续业务逻辑使用
			c.Set("requestID", rid)

			return next(c)
		}
	}
}
