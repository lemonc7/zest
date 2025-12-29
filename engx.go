package engx

import "net/http"

type Engx struct {
	router     *Router
	ErrHandler ErrHandlerFunc
}

type Map map[string]any

type HTTPError struct {
	StatusCode int
	Msg        string
	Internal   error
}

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
	e := &Engx{
		ErrHandler: DefaultErrHandlerFunc,
		router:     NewRouter(),
	}
	return e
}

func (e *Engx) addRoute(method string, pattern string, handler HandlerFunc) {
	e.router.addRoute(method, pattern, handler)
}

func (e *Engx) GET(pattern string, handler HandlerFunc) {
	e.addRoute(http.MethodGet, pattern, handler)
}

func (e *Engx) POST(pattern string, handler HandlerFunc) {
	e.addRoute(http.MethodPost, pattern, handler)
}

func (e *Engx) PUT(pattern string, handler HandlerFunc) {
	e.addRoute(http.MethodPut, pattern, handler)
}

func (e *Engx) PATCH(pattern string, handler HandlerFunc) {
	e.addRoute(http.MethodPatch, pattern, handler)
}

func (e *Engx) DELETE(pattern string, handler HandlerFunc) {
	e.addRoute(http.MethodDelete, pattern, handler)
}

func (e *Engx) Run(addr string) error {
	return http.ListenAndServe(addr, e)
}

func (e *Engx) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := NewContext(w, r)
	if err := e.router.handle(c); err != nil {
		e.ErrHandler(err, c)
	}
}
