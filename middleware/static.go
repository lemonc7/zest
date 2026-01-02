package middleware

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/lemonc7/engx"
)

// StaticConfig 静态文件中间件配置
type StaticConfig struct {
	// Root 静态文件的根目录
	Root string
	// Index 目录的默认文件名
	// 可选，默认值 "index.html"
	Index string
	// HTML5 HTML5 模式（单页应用模式）
	// 如果设置为 true，当文件不存在时，会返回 index.html
	// 可选，默认值 false
	HTML5 bool
	// Browse 是否允许目录浏览
	// 可选，默认值 false
	Browse bool
	// IgnoreBase （当 HTML5 为 true 时）忽略请求 URL 中的基础路径
	// 使得嵌套路径也能返回 index.html
	// 可选，默认值 false
	IgnoreBase bool
}

// DefaultStaticConfig 默认静态文件配置
var DefaultStaticConfig = StaticConfig{
	Root:  ".",
	Index: "index.html",
}

// Static 返回一个静态文件中间件，从指定的根目录提供静态内容
func Static(root string) engx.MiddlewareFunc {
	c := DefaultStaticConfig
	c.Root = root
	return StaticWithConfig(c)
}

// StaticWithConfig 返回一个带配置的静态文件中间件
func StaticWithConfig(config StaticConfig) engx.MiddlewareFunc {
	// 设置默认值
	if config.Root == "" {
		config.Root = "."
	}
	if config.Index == "" {
		config.Index = "index.html"
	}

	// 返回中间件函数
	return func(next engx.HandlerFunc) engx.HandlerFunc {
		return func(c *engx.Context) error {
			// ============ 步骤 1: 过滤非静态请求 ============
			// 只处理 GET 和 HEAD 请求，其他方法（POST/PUT/DELETE）直接跳过
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				return next(c)
			}

			// ============ 步骤 2: 构建文件路径 ============
			// 获取请求的 URL 路径，例如 "/css/style.css"
			path := c.Request.URL.Path

			// 安全地拼接文件路径，防止路径遍历攻击（如 "/../../../etc/passwd"）
			// filepath.Clean 会清理路径中的 .. 和 .
			// filepath.Join 会安全地拼接路径
			// 例如：config.Root = "./dist", path = "/css/style.css"
			// 结果：file = "./dist/css/style.css"
			file := filepath.Join(config.Root, filepath.Clean("/"+path))

			// ============ 步骤 3: 检查文件是否存在 ============
			info, err := os.Stat(file)
			if err != nil {
				// 3.1 文件不存在的情况
				if os.IsNotExist(err) {
					// ========== SPA 模式处理 ==========
					// 如果开启了 HTML5 模式（用于 Vue/React 单页应用）
					// 找不到文件时，返回 index.html，让前端路由接管
					if config.HTML5 {
						file = filepath.Join(config.Root, config.Index)
						// 再次检查 index.html 是否存在
						if info, err = os.Stat(file); err == nil {
							http.ServeFile(c.Writer, c.Request, file)
							return nil
						}
					}
					// 文件不存在且非 HTML5 模式，继续执行下一个 Handler
					// 可能会被其他路由处理，或最终返回 404
					return next(c)
				}

				// 3.2 其他错误（如权限错误）
				// 为了稳健性，继续执行下一个 Handler
				return next(c)
			}

			// ============ 步骤 4: 处理目录请求 ============
			if info.IsDir() {
				// 4.1 尝试查找目录下的 index 文件（如 index.html）
				indexFile := filepath.Join(file, config.Index)
				if _, err := os.Stat(indexFile); err == nil {
					// 找到了 index 文件，直接返回
					http.ServeFile(c.Writer, c.Request, indexFile)
					return nil
				}

				// 4.2 如果开启了目录浏览（Browse），列出目录内容
				if config.Browse {
					fs := http.FileServer(http.Dir(config.Root))
					fs.ServeHTTP(c.Writer, c.Request)
					return nil
				}

				// 4.3 目录且无 index 且未开启浏览 -> 继续下一个 handler
				return next(c)
			}

			// ============ 步骤 5: 返回普通文件 ============
			// http.ServeFile 会：
			// 1. 自动检测 Content-Type（如 image/png, text/css）
			// 2. 处理缓存（Last-Modified, If-Modified-Since）
			// 3. 支持 Range 请求（视频播放、断点续传）
			http.ServeFile(c.Writer, c.Request, file)
			return nil
		}
	}
}
