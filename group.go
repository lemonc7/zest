package engx

import "net/http"

// Group 路由分组
type Group struct {
	prefix      string
	middlewares []MiddlewareFunc
	engx        *Engx
}

func (g *Group) handle(method string, pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	// 拼接路由前缀
	fullPattern := g.prefix + pattern

	// 合并分组中间件和路由中间件
	finalMws := append(g.middlewares, mws...)

	g.engx.handle(method, fullPattern, handler, finalMws...)
}

// Group 创建嵌套分组
func (g *Group) Group(prefix string, mws ...MiddlewareFunc) *Group {
	return &Group{
		prefix:      g.prefix + prefix,
		middlewares: append(g.middlewares, mws...),
		engx:        g.engx,
	}
}

// Use 添加分组中间件
func (g *Group) Use(mws ...MiddlewareFunc) {
	g.middlewares = append(g.middlewares, mws...)
}

// Methods
func (g *Group) GET(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodGet, pattern, handler, mws...)
}

func (g *Group) POST(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodPost, pattern, handler, mws...)
}

func (g *Group) PUT(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodPut, pattern, handler, mws...)
}

func (g *Group) PATCH(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodPatch, pattern, handler, mws...)
}

func (g *Group) DELETE(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodDelete, pattern, handler, mws...)
}
