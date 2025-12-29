package engx

// import (
// 	"fmt"
// 	"net/http"
// 	"strings"
// )

// type ServeMux struct {
// 	http.ServeMux
// 	ErrHandlerFunc ErrHandlerFunc
// 	mws            []MiddlewareFunc
// }

// func NewServeMux() *ServeMux {
// 	return &ServeMux{
// 		ServeMux:       http.ServeMux{},
// 		ErrHandlerFunc: DefaultErrHandlerFunc,
// 	}
// }

// // 处理http请求，并且处理错误
// func (sm *ServeMux) HandleFunc(pattern string, h HandlerFunc) {
// 	// sm.ServeMux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
// 	// 	err := h(w, r)

// 	// 	if err != nil {
// 	// 		sm.ErrHandlerFunc(err, w)
// 	// 	}
// 	// })
// 	sm.ServeMux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {

// 	})
// }

// // 处理中间件，启动HTTP服务
// func (sm *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	h := http.Handler(&sm.ServeMux)

// 	for i := len(sm.mws) - 1; i >= 0; i-- {
// 		h = sm.mws[i](h)
// 	}

// 	h.ServeHTTP(w, r)
// }

// // 路由分组
// func (sm *ServeMux) Group(prefix string) *ServeMux {
// 	if len(prefix) == 0 {
// 		panic("len(prefix) must greater than 0")
// 	}

// 	if prefix[len(prefix)-1] != '/' {
// 		panic("the last char in the prefix must be /")
// 	}

// 	mux := &ServeMux{
// 		ServeMux:       http.ServeMux{},
// 		ErrHandlerFunc: sm.ErrHandlerFunc,
// 	}

// 	pre := strings.TrimSuffix(prefix, "/")
// 	sm.Handle(prefix, http.StripPrefix(pre, mux))

// 	return mux
// }

// // Methods
// func (sm *ServeMux) GET(path string, handler HandlerFunc, mw ...RouteMiddlewareFunc) {
// 	handler = use(handler, mw...)
// 	sm.HandleFunc(fmt.Sprintf("%s %s", http.MethodGet, path), handler)
// }

// func (sm *ServeMux) POST(path string, handler HandlerFunc, mw ...RouteMiddlewareFunc) {
// 	handler = use(handler, mw...)
// 	sm.HandleFunc(fmt.Sprintf("%s %s", http.MethodPost, path), handler)
// }

// func (sm *ServeMux) PUT(path string, handler HandlerFunc, mw ...RouteMiddlewareFunc) {
// 	handler = use(handler, mw...)
// 	sm.HandleFunc(fmt.Sprintf("%s %s", http.MethodPut, path), handler)
// }

// func (sm *ServeMux) PATCH(path string, handler HandlerFunc, mw ...RouteMiddlewareFunc) {
// 	handler = use(handler, mw...)
// 	sm.HandleFunc(fmt.Sprintf("%s %s", http.MethodPatch, path), handler)
// }

// func (sm *ServeMux) DELETE(path string, handler HandlerFunc, mw ...RouteMiddlewareFunc) {
// 	handler = use(handler, mw...)
// 	sm.HandleFunc(fmt.Sprintf("%s %s", http.MethodDelete, path), handler)
// }

// // 将路由中间件添加到HandlerFunc
// func use(h HandlerFunc, m ...RouteMiddlewareFunc) HandlerFunc {
// 	for i := len(m) - 1; i >= 0; i-- {
// 		h = m[i](h)
// 	}

// 	return h
// }

// func (sm *ServeMux) Use(mw ...MiddlewareFunc) {
// 	sm.mws = append(sm.mws, mw...)
// }
