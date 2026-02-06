package zest

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Zest struct {
	mux         *http.ServeMux
	ErrHandler  ErrHandlerFunc
	middlewares []MiddlewareFunc
	pool        sync.Pool
}

type Map map[string]any

type HandlerFunc func(c *Context) error

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type ErrHandlerFunc func(c *Context, err error)

var contextKey = struct{}{}

func New() *Zest {
	z := &Zest{
		ErrHandler: DefaultErrHandlerFunc,
		mux:        http.NewServeMux(),
	}
	z.pool.New = func() any {
		return NewContext(nil, nil)
	}

	// 注册全局 404 处理，利用 Go 1.22 的特性
	// 注册一个不带方法的模式会作为最后的兜底
	z.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c := r.Context().Value(contextKey).(*Context)
		c.sync(w, r)

		// 通过全局错误处理器返回标准 404
		z.ErrHandler(c, NewHTTPError(http.StatusNotFound, "not found"))
	})

	return z
}

func (z *Zest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := z.pool.Get().(*Context)
	c.reset(w, r)
	c.zest = z
	defer z.pool.Put(c)

	// 将自定义的 Context 存入上下文中
	r = r.WithContext(context.WithValue(r.Context(), contextKey, c))
	c.Request = r

	handle := func(ctx *Context) error {
		z.mux.ServeHTTP(ctx.ResponseWriter(), ctx.Request)
		return nil
	}

	// 将全局中间件应用到最外层
	handle = use(handle, z.middlewares...)

	// 错误处理
	if err := handle(c); err != nil {
		z.ErrHandler(c, err)
	}
}

func (z *Zest) handle(method string, pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	route := method + " " + pattern

	// 处理局部路由中间件
	finalHandler := use(handler, mws...)

	z.mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		// 此时能进这里的请求，已经经过了 ServeHTTP 里的全局中间件
		c := r.Context().Value(contextKey).(*Context)
		c.sync(w, r)

		if err := finalHandler(c); err != nil {
			z.ErrHandler(c, err)
		}
	})
}

func (z *Zest) GET(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodGet, pattern, handler, mws...)
}

func (z *Zest) POST(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodPost, pattern, handler, mws...)
}

func (z *Zest) PUT(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodPut, pattern, handler, mws...)
}

func (z *Zest) PATCH(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodPatch, pattern, handler, mws...)
}

func (z *Zest) DELETE(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodDelete, pattern, handler, mws...)
}

func (z *Zest) OPTIONS(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	z.handle(http.MethodOptions, pattern, handler, mws...)
}

func (z *Zest) Run(addr string) error {
	log.Printf("🚀 Zest server listening on %s\n", addr)
	return http.ListenAndServe(addr, z)
}

func (z *Zest) Use(mws ...MiddlewareFunc) {
	z.middlewares = append(z.middlewares, mws...)
}

// Group 创建路由分组
func (z *Zest) Group(prefix string, mws ...MiddlewareFunc) *Group {
	return &Group{
		prefix:      prefix,
		middlewares: mws,
		zest:        z,
	}
}

// Static 静态文件服务
// 建议直接使用 middleware.Static 中间件获得更多配置项
func (z *Zest) Static(prefix, root string) {
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

	z.GET(prefix+"{path...}", func(c *Context) error {
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
