package engx

import (
	"log"
	"net/http"
	"strings"
	"sync"
)

type Engx struct {
	mux         *http.ServeMux
	ErrHandler  ErrHandlerFunc
	middlewares []MiddlewareFunc
	pool        sync.Pool
}

type Map map[string]any

type HandlerFunc func(c *Context) error

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type ErrHandlerFunc func(err error, c *Context)

const (
	charsetUTF8 = "charset=UTF-8"
)

// Header
const (
	HeaderContentType = "Content-Type"
)

// MIME type
const (
	MIMEApplicationJSON           = "application/json"
	MIMEApplicationXML            = "application/xml"
	MIMETextPlain                 = "text/plain"
	MIMETextHTML                  = "text/html"
	MIMEApplicationXMLCharsetUTF8 = MIMEApplicationXML + "; " + charsetUTF8
	MIMETextPlainCharsetUTF8      = MIMETextPlain + "; " + charsetUTF8
	MIMETextHTMLCharsetUTF8       = MIMETextHTML + "; " + charsetUTF8
)

func New() *Engx {
	e := &Engx{
		ErrHandler: DefaultErrHandlerFunc,
		mux:        http.NewServeMux(),
	}
	e.pool.New = func() any {
		return NewContext(nil, nil)
	}

	// 注册全局 404 处理，利用 Go 1.22 的特性
	// 注册一个不带方法的模式会作为最后的兜底
	e.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c := e.pool.Get().(*Context)
		c.reset(w, r)
		defer e.pool.Put(c)

		// 通过全局错误处理器返回标准 404
		e.ErrHandler(NewHTTPError(http.StatusNotFound, "Not Found"), c)
	})

	return e
}

func (e *Engx) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mux.ServeHTTP(w, r)
}

func (e *Engx) handle(method string, pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	route := method + " " + pattern

	// 合并全局和局部路由中间件
	finalMws := append(e.middlewares, mws...)
	finalHandler := use(handler, finalMws...)

	e.mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		c := e.pool.Get().(*Context)
		c.reset(w, r)
		defer e.pool.Put(c)

		if err := finalHandler(c); err != nil {
			e.ErrHandler(err, c)
		}
	})
}

func (e *Engx) GET(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodGet, pattern, handler, mws...)
}

func (e *Engx) POST(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodPost, pattern, handler, mws...)
}

func (e *Engx) PUT(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodPut, pattern, handler, mws...)
}

func (e *Engx) PATCH(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodPatch, pattern, handler, mws...)
}

func (e *Engx) DELETE(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodDelete, pattern, handler, mws...)
}

func (e *Engx) OPTIONS(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	e.handle(http.MethodOptions, pattern, handler, mws...)
}

func (e *Engx) Run(addr string) error {
	log.Printf("🚀 Engx server listening on %s\n", addr)
	return http.ListenAndServe(addr, e)
}

func (e *Engx) Use(mws ...MiddlewareFunc) {
	e.middlewares = append(e.middlewares, mws...)
}

// Group 创建路由分组
func (e *Engx) Group(prefix string, mws ...MiddlewareFunc) *Group {
	return &Group{
		prefix:      prefix,
		middlewares: mws,
		engx:        e,
	}
}

// Static 静态文件服务
// 建议直接使用 middleware.Static 中间件获得更多配置项
func (e *Engx) Static(prefix, root string) {
	if prefix == "" {
		prefix = "/"
	}
	// 确保 prefix 以 / 开头
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	// 确保 prefix 以 / 结尾
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	fileServer := http.FileServer(http.Dir(root))
	handler := http.StripPrefix(prefix, fileServer)

	e.GET(prefix+"{path...}", func(c *Context) error {
		handler.ServeHTTP(c.ResponseWriter(), c.Request)
		return nil
	})
}

func use(handler HandlerFunc, mws ...MiddlewareFunc) HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
