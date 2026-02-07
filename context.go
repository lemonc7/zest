package zest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
	zest     *Zest
}

// Response嵌入http.ResponseWriter 并提供了状态和大小追踪
type Response struct {
	http.ResponseWriter
	Status    int
	Size      int64
	Committed bool
}

func (r *Response) WriteHeader(code int) {
	if r.Committed {
		return
	}
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
	r.Committed = true
}

func (r *Response) Write(b []byte) (int, error) {
	if !r.Committed {
		if r.Status == 0 {
			r.Status = http.StatusOK
		}
		r.WriteHeader(r.Status)
	}
	n, err := r.ResponseWriter.Write(b)
	r.Size += int64(n)
	return n, err
}

func (r *Response) WriteString(s string) (int, error) {
	if !r.Committed {
		if r.Status == 0 {
			r.Status = http.StatusOK
		}
		r.WriteHeader(r.Status)
	}
	n, err := io.WriteString(r.ResponseWriter, s)
	r.Size += int64(n)
	return n, err
}

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	c := &Context{}
	c.reset(w, r)
	return c
}

func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.response.ResponseWriter = w
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
	c.zest = nil
}

func (c *Context) sync(w http.ResponseWriter, r *http.Request) {
	c.Request = r
	c.response.ResponseWriter = w
	if r != nil {
		c.Path = r.URL.Path
		c.Method = r.Method
	}
}

func (c *Context) Context() context.Context {
	return c.Request.Context()
}

// Error 触发全局错误处理器
// 这允许中间件在链中处理错误，而不是等到最外层
func (c *Context) Error(err error) {
	if c.zest != nil && c.zest.ErrHandler != nil {
		c.zest.ErrHandler(c, err)
	}
}

// 路由参数，依赖 Go 1.22+ 的 r.PathValue
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

// Params Query参数
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// Cookie 返回指定名称的 Cookie
func (c *Context) Cookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// SetCookie 设置 Cookie
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.response.ResponseWriter, cookie)
}

// FormValue 返回指定名称的表单参数
func (c *Context) FormValue(name string) string {
	return c.Request.FormValue(name)
}

// FormFile 返回指定名称的上传文件
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	_, fh, err := c.Request.FormFile(name)
	return fh, err
}

// MultipartForm 返回解析后的 MultipartForm
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(32 << 20) // 默认 32MB
	return c.Request.MultipartForm, err
}

func (c *Context) SetStatus(statusCode int) {
	c.response.WriteHeader(statusCode)
}

func (c *Context) SetHeader(key string, value string) {
	c.response.Header().Set(key, value)
}

// Response 返回 Response 对象（用于获取 Size 等信息）
func (c *Context) Response() *Response {
	return &c.response
}

// Writer 返回底层的 ResponseWriter
func (c *Context) ResponseWriter() http.ResponseWriter {
	return c.response.ResponseWriter
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
	// 使用 strings.Cut 避免 strings.Split 产生的切片分配
	if ip, _, found := strings.Cut(clientIP, ","); found {
		clientIP = ip
	}
	clientIP = strings.TrimSpace(clientIP)

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
	http.ServeFile(c.response.ResponseWriter, c.Request, filepath)
}

// Attachment 用于提供文件下载，并指定下载文件名
func (c *Context) Attachment(file string, name string) {
	// 核心区别在这里：设置 Content-Disposition 为 attachment
	// 这明确告诉浏览器："不要尝试渲染这个内容，直接当作附件下载"
	// filename=... 指定了用户保存时默认显示的文件名
	c.SetHeader("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(name))
	// 同样复用 ServeFile 来处理文件流传输
	http.ServeFile(c.response.ResponseWriter, c.Request, file)
}
