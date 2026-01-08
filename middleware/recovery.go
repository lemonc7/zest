package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/lemonc7/engx"
)

// RecoveryConfig Recovery 中间件配置
type RecoveryConfig struct {
	// Skip 跳过处理的计数器（用于 runtime.Callers）
	// 默认 3
	Skip int
	// LogFunc 自定义日志打印函数
	// 默认为 log.Printf
	LogFunc func(format string, v ...any)
}

// DefaultRecoveryConfig 默认配置
var DefaultRecoveryConfig = RecoveryConfig{
	Skip:    3,
	LogFunc: log.Printf,
}

// Recovery 返回一个中间件，用于捕获 panic 并恢复，防止服务器崩溃
// 如果发生 panic，会记录堆栈信息，并返回 500 错误以便后续中间件（如 Logger）和全局错误处理器处理
func Recovery(config ...RecoveryConfig) engx.MiddlewareFunc {
	cfg := DefaultRecoveryConfig
	if len(config) > 0 {
		userCfg := config[0]
		if userCfg.Skip > 0 {
			cfg.Skip = userCfg.Skip
		}
		if userCfg.LogFunc != nil {
			cfg.LogFunc = userCfg.LogFunc
		}
	}

	return func(next engx.HandlerFunc) engx.HandlerFunc {
		return func(c *engx.Context) (err error) {
			// ============ 使用 defer + recover 捕获 panic ============
			defer func() {
				if r := recover(); r != nil {
					// ========== 步骤 1: 检查是否是网络连接中断 ==========
					var brokenPipe bool
					if ne, ok := r.(netError); ok {
						errMsg := strings.ToLower(ne.Error())
						if strings.Contains(errMsg, "broken pipe") ||
							strings.Contains(errMsg, "connection reset by peer") {
							brokenPipe = true
						}
					}

					// ========== 步骤 2: 获取堆栈信息 ==========
					// 如果不是 Broken Pipe，或者是 Broken Pipe 但我们也想看一点信息（通常 BrokenPipe 不需要看堆栈）
					// 这里保持逻辑：BrokenPipe 不打印堆栈
					if !brokenPipe {
						trace := trace(cfg.Skip)
						// 使用配置的 LogFunc 打印到 stderr 或文件
						cfg.LogFunc("[Recovery] panic recovered:\n%v\n%s", r, trace)
					}

					// ========== 步骤 3: 构造错误返回 ==========
					if brokenPipe {
						// 如果是网络断开，返回 nil 终止后续处理，也不需要写响应
						err = nil
						return
					}

					// 将 panic 转换为 error 返回
					// 这样 Logger 中间件可以记录这个 Error
					// Engx 核心会捕获这个 Error 并调用 ErrHandler 返回 500 JSON
					// err隐式返回
					err = engx.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("%v", r))

					// 如果响应头还没写入，Engx ErrHandler 会负责写入
					// 如果响应已经部分写入了（c.Response().Committed），那也没办法了，只能让客户端接收截断的数据
				}
			}()

			// ============ 执行实际的 Handler ============
			return next(c)
		}
	}
}

// netError 网络错误接口
type netError interface {
	Error() string
}

// trace 获取堆栈跟踪信息
func trace(skip int) string {
	var pcs [32]uintptr
	n := runtime.Callers(skip, pcs[:])
	var b strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&b, "\t%s:%d\n", frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}
