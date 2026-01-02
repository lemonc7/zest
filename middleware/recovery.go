package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/lemonc7/engx"
)

// Recovery 返回一个中间件，用于捕获 panic 并恢复，防止服务器崩溃
// 如果发生 panic，会写入 500 错误响应并记录堆栈信息
func Recovery() engx.MiddlewareFunc {
	return func(next engx.HandlerFunc) engx.HandlerFunc {
		return func(c *engx.Context) error {
			// ============ 使用 defer + recover 捕获 panic ============
			// defer 确保在函数返回前执行，即使发生 panic
			defer func() {
				if err := recover(); err != nil {
					// ========== 步骤 1: 检查是否是网络连接中断 ==========
					// 如果连接已断开（如用户关闭浏览器），没必要打印完整堆栈
					var brokenPipe bool
					if ne, ok := err.(netError); ok {
						// 检查错误信息中是否包含 "broken pipe" 或 "connection reset"
						errMsg := strings.ToLower(ne.Error())
						if strings.Contains(errMsg, "broken pipe") ||
							strings.Contains(errMsg, "connection reset by peer") {
							brokenPipe = true
						}
					}

					// ========== 步骤 2: 处理连接中断的情况 ==========
					if brokenPipe {
						// 连接已断开，无法写入响应，直接返回
						// 不打印堆栈，因为这不是程序错误，而是网络问题
						return
					}

					// ========== 步骤 3: 打印 panic 信息和堆栈跟踪 ==========
					// trace(4) 会跳过前 4 个栈帧（runtime 内部调用）
					trace := trace(4)
					fmt.Printf("[Recovery] panic recovered:\n%s\n%s\n", err, trace)

					// ========== 步骤 4: 返回 500 错误给客户端 ==========
					// 先设置状态码
					c.SetStatus(http.StatusInternalServerError)
					// 再返回错误信息
					c.String(http.StatusInternalServerError, "Internal Server Error")
				}
			}()

			// ============ 执行实际的 Handler ============
			// 如果这里发生 panic，会被上面的 defer recover 捕获
			return next(c)
		}
	}
}

// netError 网络错误接口，用于类型断言
// 只要实现了 Error() 方法的类型都可以匹配
type netError interface {
	Error() string
}

// trace 获取堆栈跟踪信息
// skip: 跳过的栈帧数量，用于过滤掉 runtime 内部调用
func trace(skip int) string {
	// ========== 步骤 1: 获取调用栈 ==========
	var pcs [32]uintptr // 最多记录 32 层调用栈
	// runtime.Callers 获取当前调用栈的程序计数器（PC）
	// skip=4 表示跳过：callers -> trace -> defer func -> recover
	n := runtime.Callers(skip, pcs[:])

	// ========== 步骤 2: 构建堆栈信息字符串 ==========
	var b strings.Builder
	// runtime.CallersFrames 将 PC 转换为可读的栈帧信息
	frames := runtime.CallersFrames(pcs[:n])

	// ========== 步骤 3: 遍历所有栈帧 ==========
	for {
		frame, more := frames.Next()
		// 打印每一层调用的文件路径和行号
		// 例如：/path/to/handler.go:42
		fmt.Fprintf(&b, "\t%s:%d\n", frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}
