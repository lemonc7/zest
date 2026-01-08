package engx

import (
	"fmt"
	"net/http"
)

type HTTPError struct {
	StatusCode int
	Msg        any
	Internal   error
}

func DefaultErrHandlerFunc(err error, c *Context) {
	if c.WroteHeader() {
		return
	}
	if he, ok := err.(*HTTPError); ok {
		c.JSON(he.StatusCode, Map{"error": he.Msg})
		return
	}
	c.JSON(http.StatusInternalServerError, Map{"error": err.Error()})
}

func NewHTTPError(statusCode int, msg any) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Msg:        msg,
	}
}

func (e *HTTPError) Error() string {
	if e.Internal == nil {
		return fmt.Sprintf("code=%d, message=%v", e.StatusCode, e.Msg)
	}
	return fmt.Sprintf("code=%d, message=%v, error=%v", e.StatusCode, e.Msg, e.Internal)
}

func (e *HTTPError) SetInternal(err error) *HTTPError {
	e.Internal = err
	return e
}
