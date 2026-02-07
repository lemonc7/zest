package middleware

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata"

	"github.com/lemonc7/zest"
)

// LoggerConfig 日志中间件配置
type LoggerConfig struct {
	// Skip 判断是否跳过日志记录的函数
	// 返回 true 则不记录
	Skip func(c *zest.Context) bool
	// Formatter 自定义日志格式化函数
	// 接收 LogParam 参数，返回格式化后的字符串
	Formatter func(param LogParam) string
	// Output 日志输出目标
	// 默认为 os.Stdout
	Output io.Writer
	// 时区，默认为Asia/Shanghai
	TZ *time.Location
}

// LogParam 日志参数，包含请求的所有关键信息
type LogParam struct {
	TimeStamp time.Time     // 请求完成时间
	Status    int           // HTTP 状态码
	Latency   time.Duration // 请求耗时
	Size      int64         // 响应大小（字节）
	RequestID string        // 请求唯一 ID
	ClientIP  string        // 客户端 IP
	Method    string        // HTTP 方法（GET/POST/etc）
	Path      string        // 请求路径（包含 query 参数）
	Error     error         // 如果 handler 返回了错误
}

// DefaultLoggerConfig 默认日志配置
var DefaultLoggerConfig = LoggerConfig{
	Formatter: defaultLogFormatter,
	Output:    os.Stdout,
	TZ:        mustLoadLocation("Asia/Shanghai"),
}

const (
	cyan    = "\033[96m"
	green   = "\033[92m"
	yellow  = "\033[93m"
	red     = "\033[91m"
	blue    = "\033[94m"
	magenta = "\033[95m"
	reset   = "\033[0m"
)

// defaultLogFormatter 默认的日志格式化函数
func defaultLogFormatter(param LogParam) string {
	var b strings.Builder
	b.Grow(128) // 预分配 buffer，避免由于扩容产生的多次内存分配

	// 格式化 RequestID，如果为空则显示 -
	rid := param.RequestID
	if rid == "" {
		rid = "-"
	} else if len(rid) > 8 {
		rid = rid[:8]
	}

	// [ID]
	b.WriteString("[")
	b.WriteString(rid)
	b.WriteString("] ")

	// Emoji
	b.WriteString(getStatusEmoji(param.Status))
	b.WriteString(" ")

	// Time
	b.WriteString(param.TimeStamp.Format("2006/01/02 15:04:05"))
	b.WriteString(" | ")

	// Status with Color
	b.WriteString(getStatusColor(param.Status))
	b.WriteString(strconv.Itoa(param.Status)) // 使用 Itoa 替代 fmt.Sprintf("%3d")
	b.WriteString(reset)
	b.WriteString(" | ")

	// Method with Color
	b.WriteString(getMethodColor(param.Method))
	b.WriteString(param.Method)
	b.WriteString(reset)
	b.WriteString(" | ")

	// Latency
	b.WriteString(formatLatency(param.Latency))
	b.WriteString(" | ")

	// Size
	b.WriteString(formatSize(param.Size))
	b.WriteString(" | ")

	// IP
	b.WriteString(param.ClientIP)
	b.WriteString(" | ")

	// Path
	b.WriteString(param.Path)

	// Error
	if param.Error != nil {
		b.WriteString(" | ")
		b.WriteString(red)
		b.WriteString("Error: ")
		b.WriteString(param.Error.Error())
		b.WriteString(reset)
	}

	b.WriteString("\n")
	return b.String()
}

func formatSize(s int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(s)
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", s)
	}
	return fmt.Sprintf("%.2f %s", size, units[i])
}

func formatLatency(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2f s", float64(d)/float64(time.Second))
	case d >= time.Millisecond:
		return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2f µs", float64(d)/float64(time.Microsecond))
	}
}

// Logger 返回一个日志中间件，记录所有 HTTP 请求
func Logger(config ...LoggerConfig) zest.MiddlewareFunc {
	// ... (Config logic remains unchanged) ...
	cfg := DefaultLoggerConfig

	// 如果用户提供了自定义配置，使用用户配置
	if len(config) > 0 {
		userCfg := config[0]
		if userCfg.Skip != nil {
			cfg.Skip = userCfg.Skip
		}
		if userCfg.Formatter != nil {
			cfg.Formatter = userCfg.Formatter
		}
		if userCfg.Output != nil {
			cfg.Output = userCfg.Output
		}
		if userCfg.TZ != nil {
			cfg.TZ = userCfg.TZ
		}
	}

	// 返回实际的中间件函数
	return func(next zest.HandlerFunc) zest.HandlerFunc {
		return func(c *zest.Context) error {
			if cfg.Skip != nil && cfg.Skip(c) {
				return next(c)
			}

			// ============ 步骤 1: 记录开始时间 ============
			start := time.Now()

			// ============ 步骤 2: 保存原始路径和查询参数 ============
			path := c.Request.URL.Path
			raw := c.Request.URL.RawQuery

			// ============ 步骤 3: 执行实际的 Handler ============
			err := next(c)

			// ============ 步骤 4: 如果有错误，先调用全局错误处理器 ============
			// 这样可以确保日志中记录的 status code 是正确的错误状态码
			if err != nil {
				c.Error(err)
			}

			// ============ 步骤 5: 拼接完整路径（包含查询参数）============
			if raw != "" {
				path = path + "?" + raw
			}

			// ============ 步骤 6: 收集日志参数 ============
			// 尝试获取 RequestID
			var rid string
			if v := c.Get("requestID"); v != nil {
				if id, ok := v.(string); ok {
					rid = id
				}
			}

			// 如果有错误，尝试解包获取内部错误
			var internalErr error
			var he *zest.HTTPError
			if errors.As(err, &he) && he.Unwrap() != nil {
				internalErr = he.Unwrap()
			} else {
				internalErr = err
			}

			param := LogParam{
				TimeStamp: time.Now().In(cfg.TZ),
				Status:    c.Response().Status,
				Latency:   time.Since(start),
				Size:      c.Response().Size,
				RequestID: rid,
				ClientIP:  c.ClientIP(),
				Method:    c.Method,
				Path:      path,
				Error:     internalErr,
			}

			// ============ 步骤 7: 格式化并输出日志 ============
			logStr := cfg.Formatter(param)
			fmt.Fprint(cfg.Output, logStr)

			// ============ 步骤 8: 返回原始错误 ============
			// 即使已经通过 c.Error() 处理过，仍然返回原始错误
			// 这样上层中间件可以继续处理，而全局错误处理器会检查 Committed 避免重复写入
			return err
		}
	}
}

func getStatusColor(code int) string {
	switch {
	case code >= 200 && code < 300:
		return green
	case code >= 300 && code < 400:
		return yellow
	default:
		return red
	}
}

func getMethodColor(method string) string {
	switch method {
	case "GET":
		return cyan
	case "POST":
		return green
	case "PUT":
		return yellow
	case "DELETE":
		return red
	case "PATCH":
		return magenta
	case "HEAD":
		return blue
	default:
		return reset
	}
}

// getStatusEmoji 根据状态码返回对应的 Emoji
func getStatusEmoji(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "🟢"
	case code >= 300 && code < 400:
		return "🟡"
	case code >= 400 && code < 500:
		return "🟠"
	default:
		return "🔴"
	}
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}
