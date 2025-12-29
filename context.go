package engx

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Context struct {
	Writer     http.ResponseWriter
	Request    *http.Request
	Path       string
	Method     string
	Params     map[string]string
	StatusCode int
	store      Map
	lock       sync.RWMutex
}

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Writer:  w,
		Request: r,
		Path:    r.URL.Path,
		Method:  r.Method,
	}
}

// 依赖 Go 1.22+ 的 r.PathValue
func (c *Context) Param(key string) string {
	return c.Request.PathValue(key)
}

func (c *Context) QueryParams(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) FormParams(key string) string {
	return c.Request.FormValue(key)
}

func (c *Context) SetStatus(statusCode int) {
	c.StatusCode = statusCode
	c.Writer.WriteHeader(statusCode)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) JSON(status int, data any) error {
	c.SetHeader(HeaderContentType, MIMEApplicationJSON)
	c.SetStatus(status)
	return json.NewEncoder(c.Writer).Encode(data)
}

func (c *Context) String(status int, s string) error {
	c.SetHeader(HeaderContentType, MIMETextPlainCharsetUTF8)
	c.SetStatus(status)
	_, err := c.Writer.Write([]byte(s))
	return err
}

func (c *Context) HTML(status int, html string) error {
	c.SetHeader(HeaderContentType, MIMETextHTMLCharsetUTF8)
	c.SetStatus(status)
	_, err := c.Writer.Write([]byte(html))
	return err
}

func (c *Context) Set(key string, val any) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.store == nil {
		c.store = make(Map)
	}

	c.store[key] = val
}

func (c *Context) Get(key string) any {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.store[key]
}
