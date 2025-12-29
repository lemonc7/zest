package engx

import (
	"net/http"
)

type Engx struct {
	mux         *http.ServeMux
	ErrHandler  ErrHandlerFunc
	middlewares []MiddlewareFunc
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
	MIMEApplicationJSON      = "application/json"
	MIMETextPlain            = "text/plain"
	MIMETextHTML             = "text/html"
	MIMETextPlainCharsetUTF8 = MIMETextPlain + "; " + charsetUTF8
	MIMETextHTMLCharsetUTF8  = MIMETextHTML + "; " + charsetUTF8
)

func New() *Engx {
	return &Engx{
		ErrHandler: DefaultErrHandlerFunc,
		mux:        http.NewServeMux(),
	}
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
		c := NewContext(w, r)
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

func (e *Engx) Run(addr string) error {
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

func use(handler HandlerFunc, mws ...MiddlewareFunc) HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
