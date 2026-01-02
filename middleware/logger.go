package middleware

import (
	"log"
	"time"

	"github.com/lemonc7/engx"
)

// LoggerConfig 日志中间件配置
type LoggerConfig struct {
	// Formatter 自定义日志格式化函数
	// 接收 LogParam 参数，返回格式化后的字符串
	Formatter func(param LogParam) string
	// Output 自定义日志输出函数
	// 接收格式化后的日志字符串，可以输出到文件、数据库等
	Output func(string)
}

// LogParam 日志参数，包含请求的所有关键信息
type LogParam struct {
	TimeStamp  time.Time     // 请求完成时间
	StatusCode int           // HTTP 状态码
	Latency    time.Duration // 请求耗时
	ClientIP   string        // 客户端 IP
	Method     string        // HTTP 方法（GET/POST/etc）
	Path       string        // 请求路径（包含 query 参数）
	Error      error         // 如果 handler 返回了错误
}

// DefaultLoggerConfig 默认日志配置
var DefaultLoggerConfig = LoggerConfig{
	Formatter: defaultLogFormatter,
	Output:    func(s string) { log.Print(s) },
}

// defaultLogFormatter 默认的日志格式化函数
// 输出格式：🟢 2024/01/01 - 12:00:00 | GET /api/users | 5.2ms | 127.0.0.1
func defaultLogFormatter(param LogParam) string {
	return getStatusEmoji(param.StatusCode) + " " +
		param.TimeStamp.Format("2006/01/02 - 15:04:05") + " | " +
		param.Method + " " +
		param.Path + " | " +
		param.Latency.String() + " | " +
		param.ClientIP
}

// getStatusEmoji 根据状态码返回对应的 Emoji
// 2xx 成功 -> 🟢  3xx 重定向 -> 🟡  4xx 客户端错误 -> 🟠  5xx 服务器错误 -> 🔴
func getStatusEmoji(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "🟢" // 成功
	case code >= 300 && code < 400:
		return "🟡" // 重定向
	case code >= 400 && code < 500:
		return "🟠" // 客户端错误（如 404, 403）
	default:
		return "🔴" // 服务器错误（如 500）
	}
}

// Logger 返回一个日志中间件，记录所有 HTTP 请求
func Logger(config ...LoggerConfig) engx.MiddlewareFunc {
	// 使用默认配置
	cfg := DefaultLoggerConfig

	// 如果用户提供了自定义配置，使用用户配置
	if len(config) > 0 {
		cfg = config[0]
		// 如果用户没有提供 Formatter，使用默认格式化函数
		if cfg.Formatter == nil {
			cfg.Formatter = defaultLogFormatter
		}
		// 如果用户没有提供 Output，使用默认输出（标准输出）
		if cfg.Output == nil {
			cfg.Output = func(s string) { log.Print(s) }
		}
	}

	// 返回实际的中间件函数
	return func(next engx.HandlerFunc) engx.HandlerFunc {
		return func(c *engx.Context) error {
			// ============ 步骤 1: 记录开始时间 ============
			start := time.Now()

			// ============ 步骤 2: 保存原始路径和查询参数 ============
			// path: /api/users
			path := c.Request.URL.Path
			// raw: page=1&size=10
			raw := c.Request.URL.RawQuery

			// ============ 步骤 3: 执行实际的 Handler ============
			// 这里会调用路由处理函数，以及后续的中间件
			err := next(c)

			// ============ 步骤 4: 拼接完整路径（包含查询参数）============
			// 如果有查询参数，拼接成 /api/users?page=1&size=10
			if raw != "" {
				path = path + "?" + raw
			}

			// ============ 步骤 5: 收集日志参数 ============
			param := LogParam{
				TimeStamp:  time.Now(),        // 请求完成时间
				StatusCode: c.StatusCode,      // HTTP 状态码（如 200, 404, 500）
				Latency:    time.Since(start), // 计算请求耗时
				ClientIP:   c.ClientIP(),      // 获取客户端真实 IP
				Method:     c.Method,          // HTTP 方法
				Path:       path,              // 完整路径（含查询参数）
				Error:      err,               // Handler 返回的错误（如果有）
			}

			// ============ 步骤 6: 格式化并输出日志 ============
			logStr := cfg.Formatter(param) // 调用格式化函数
			cfg.Output(logStr)             // 调用输出函数

			// ============ 步骤 7: 返回原始错误 ============
			// 重要！必须返回 err，让错误继续向上传递
			return err
		}
	}
}
