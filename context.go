package engx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Context struct {
	response Response
	Request  *http.Request
	Path     string
	Method   string
	store    Map
}

// Response 包装了 http.ResponseWriter 并提供了状态和大小追踪
type Response struct {
	Writer    http.ResponseWriter
	Status    int
	Size      int64
	Committed bool
}

func (r *Response) Header() http.Header {
	return r.Writer.Header()
}

func (r *Response) WriteHeader(code int) {
	if r.Committed {
		return
	}
	r.Status = code
	r.Writer.WriteHeader(code)
	r.Committed = true
}

func (r *Response) Write(b []byte) (int, error) {
	if !r.Committed {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.Writer.Write(b)
	r.Size += int64(n)
	return n, err
}

func (r *Response) WriteString(s string) (int, error) {
	if !r.Committed {
		r.WriteHeader(http.StatusOK)
	}
	n, err := io.WriteString(r.Writer, s)
	r.Size += int64(n)
	return n, err
}

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	c := &Context{}
	c.reset(w, r)
	return c
}

func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.response.Writer = w
	c.response.Status = http.StatusOK
	c.response.Size = 0
	c.response.Committed = false

	c.Request = r
	if r != nil {
		c.Path = r.URL.Path
		c.Method = r.Method
	} else {
		c.Path = ""
		c.Method = ""
	}

	// 只在 store 不为空时进行清理
	if c.store != nil {
		clear(c.store)
	}
}

func (c *Context) Context() context.Context {
	return c.Request.Context()
}

// 依赖 Go 1.22+ 的 r.PathValue
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) SetStatus(statusCode int) {
	c.response.WriteHeader(statusCode)
}

func (c *Context) SetHeader(key string, value string) {
	c.response.Header().Set(key, value)
}

func (c *Context) WroteHeader() bool {
	return c.response.Committed
}

// Response 返回 Response 对象（用于获取 Size 等信息）
func (c *Context) Response() *Response {
	return &c.response
}

// Writer 返回底层的 ResponseWriter
func (c *Context) ResponseWriter() http.ResponseWriter {
	return c.response.Writer
}

func (c *Context) JSON(status int, data any) error {
	c.SetHeader(HeaderContentType, MIMEApplicationJSON)
	c.SetStatus(status)
	return json.NewEncoder(&c.response).Encode(data)
}

func (c *Context) String(status int, s string) error {
	c.SetHeader(HeaderContentType, MIMETextPlainCharsetUTF8)
	c.SetStatus(status)
	_, err := c.response.WriteString(s)
	return err
}

func (c *Context) HTML(status int, html string) error {
	c.SetHeader(HeaderContentType, MIMETextHTMLCharsetUTF8)
	c.SetStatus(status)
	_, err := c.response.WriteString(html)
	return err
}

func (c *Context) Set(key string, val any) {
	if c.store == nil {
		c.store = make(Map)
	}
	c.store[key] = val
}

func (c *Context) Get(key string) any {
	return c.store[key]
}

func (c *Context) NoContent(status int) error {
	c.SetStatus(status)
	return nil
}

func (c *Context) Redirect(status int, url string) error {
	if status < 300 || status > 399 {
		return fmt.Errorf("invalid status code")
	}
	c.SetHeader("Location", url)
	c.SetStatus(status)
	return nil
}

// ClientIP 尝试获取客户端的真实 IP
func (c *Context) ClientIP() string {
	// 1. 优先检查 X-Forwarded-For
	// 这是最标准的代理透传 Header，格式通常是：ClientIP, Proxy1, Proxy2...
	clientIP := c.Request.Header.Get("X-Forwarded-For")
	// 只取第一个 IP（最左边的），因为那才是原始客户端的 IP
	// strings.TrimSpace 防止有空格干扰
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])

	// 2. 如果没取到，检查 X-Real-Ip
	// 这是一个非标准 Header，但在 Nginx 中非常常用
	if clientIP == "" {
		clientIP = strings.TrimSpace(c.Request.Header.Get("X-Real-Ip"))
	}
	if clientIP != "" {
		return clientIP
	}

	// 3. 最后兜底：使用直接连接的 RemoteAddr
	// RemoteAddr 格式通常是 "IP:Port"（例如 127.0.0.1:54321）
	// 所以需要用 net.SplitHostPort 去掉端口号
	if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

// File 用于提供文件下载
func (c *Context) File(filepath string) {
	// http.ServeFile 是 Go 标准库提供的强大函数：
	// 1. 自动检测文件的 Content-Type (如 image/png) 并设置 Header
	// 2. 处理 Last-Modified 和 If-Modified-Since (支持浏览器缓存！)
	// 3. 支持 Range 请求 (视频拖动播放、断点续传)
	// 4. 安全地读取文件流写入 Response
	http.ServeFile(c.response.Writer, c.Request, filepath)
}

// Attachment 用于提供文件下载，并指定下载文件名
func (c *Context) Attachment(file string, name string) {
	// 核心区别在这里：设置 Content-Disposition 为 attachment
	// 这明确告诉浏览器："不要尝试渲染这个内容，直接当作附件下载"
	// filename=... 指定了用户保存时默认显示的文件名
	c.SetHeader("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(name))
	// 同样复用 ServeFile 来处理文件流传输
	http.ServeFile(c.response.Writer, c.Request, file)
}
