package middleware

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/lemonc7/zest"
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
	// Filesystem 提供对静态内容的访问
	// 可选，默认为 http.Dir(config.Root)
	Filesystem http.FileSystem
}

const dirListHtml = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ .Name }}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; padding: 40px; background: #f9f9fb; color: #333; }
    header { font-size: 28px; font-weight: 600; margin-bottom: 30px; color: #1a1a1a; }
    ul { list-style: none; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 15px; }
    li { background: #fff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); transition: transform 0.2s, box-shadow 0.2s; }
    li:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
    li a { display: block; padding: 15px; text-decoration: none; color: #3b82f6; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .dir { color: #ec4899; font-weight: 500; }
    .file { color: #6366f1; }
    .size { font-size: 12px; color: #94a3b8; margin-left: 8px; }
  </style>
</head>
<body>
  <header>目录: {{ .Name }}</header>
  <ul>
    {{ range .Files }}
    <li>
      {{ if .IsDir }}
        <a class="dir" href="{{ .Name }}/">{{ .Name }}/</a>
      {{ else }}
        <a class="file" href="{{ .Name }}">{{ .Name }}</a>
        <span class="size">{{ .Size }}</span>
      {{ end }}
    </li>
    {{ end }}
  </ul>
</body>
</html>
`

// Static 返回一个带配置的静态文件中间件
func Static(config StaticConfig) zest.MiddlewareFunc {
	// 默认配置初始化
	if config.Root == "" {
		config.Root = "."
	}
	if config.Index == "" {
		config.Index = "index.html"
	}
	if config.Filesystem == nil {
		config.Filesystem = http.Dir(config.Root)
		config.Root = "."
	}

	// 预加载模板
	t, tErr := template.New("dirlist").Parse(dirListHtml)
	if tErr != nil {
		panic(fmt.Errorf("zest: static middleware template error: %w", tErr))
	}

	return func(next zest.HandlerFunc) zest.HandlerFunc {
		return func(c *zest.Context) error {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				return next(c)
			}

			// 获取并解码 URL 路径
			reqPath := c.Request.URL.Path
			p, err := url.PathUnescape(reqPath)
			if err != nil {
				return next(c)
			}

			// 使用 path.Clean 确保 URL 路径安全
			name := path.Join(config.Root, path.Clean("/"+p))

			file, err := config.Filesystem.Open(name)
			if err != nil {
				// 文件不存在，交给后续路由处理（可能是 API 路由）
				if err := next(c); err == nil {
					return nil
				}

				// 如果后续路由也处理失败（返回了 404），且开启了 HTML5 模式，则尝试返回 index.html
				// 这对于 SPA (单页应用) 前端路由非常重要
				var he *zest.HTTPError
				if config.HTML5 && (os.IsNotExist(err) || (errors.As(err, &he) && he.Code == http.StatusNotFound)) {
					file, err = config.Filesystem.Open(path.Join(config.Root, config.Index))
					if err != nil {
						// index.html 也不存在，那只能返回最初的 404 错误了
						return next(c)
					}
				} else {
					return next(c)
				}
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return next(c)
			}

			if info.IsDir() {
				// 尝试目录下的 index.html
				indexName := path.Join(name, config.Index)
				indexFile, err := config.Filesystem.Open(indexName)
				if err == nil {
					defer indexFile.Close()
					if indexInfo, err := indexFile.Stat(); err == nil {
						http.ServeContent(c.ResponseWriter(), c.Request, indexInfo.Name(), indexInfo.ModTime(), indexFile)
						return nil
					}
				}

				// 开启目录浏览
				if config.Browse {
					return listDir(t, name, file, c)
				}
				return next(c)
			}

			http.ServeContent(c.ResponseWriter(), c.Request, info.Name(), info.ModTime(), file)
			return nil
		}
	}
}

func listDir(t *template.Template, name string, dir http.File, c *zest.Context) error {
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	c.SetHeader(zest.HeaderContentType, zest.MIMETextHTMLCharsetUTF8)
	c.SetStatus(http.StatusOK)

	data := struct {
		Name  string
		Files []any
	}{
		Name: name,
	}

	for _, f := range files {
		data.Files = append(data.Files, struct {
			Name  string
			IsDir bool
			Size  string
		}{
			Name:  f.Name(),
			IsDir: f.IsDir(),
			Size:  formatSize(f.Size()),
		})
	}
	return t.Execute(c.Response(), data)
}
