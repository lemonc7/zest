package zest

import (
	"net/http"
	"strings"
)

// Group 路由分组
type Group struct {
	prefix      string
	middlewares []MiddlewareFunc
	zest        *Zest
}

func (g *Group) handle(method string, pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	// 拼接路由前缀，确保路径规范化
	fullPattern := joinPath(g.prefix, pattern)

	// 合并分组中间件和路由中间件
	finalMws := append(g.middlewares, mws...)

	g.zest.handle(method, fullPattern, handler, finalMws...)
}

func joinPath(prefix, pattern string) string {
	if prefix == "" {
		return pattern
	}
	if pattern == "" {
		return prefix
	}
	// 手动拼接，避免 url.JoinPath 转义 {} 等特殊字符
	final := strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(pattern, "/")
	return final
}

// Group 创建嵌套分组
func (g *Group) Group(prefix string, mws ...MiddlewareFunc) *Group {
	return &Group{
		prefix:      g.prefix + prefix,
		middlewares: append(g.middlewares, mws...),
		zest:        g.zest,
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

// Static 在分组内提供静态文件服务
func (g *Group) Static(prefix, root string) {
	// 拼接分组前缀
	fullPrefix := joinPath(g.prefix, prefix)
	g.zest.Static(fullPrefix, root)
}

func (g *Group) OPTIONS(pattern string, handler HandlerFunc, mws ...MiddlewareFunc) {
	g.handle(http.MethodOptions, pattern, handler, mws...)
}
