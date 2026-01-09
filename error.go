package zest

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
	// 响应已经提交，直接返回
	if c.Response().Committed {
		return
	}

	var status int
	var errMsg string
	if he, ok := err.(*HTTPError); ok {
		status = he.StatusCode
		switch msg := he.Msg.(type) {
		case error:
			errMsg = msg.Error()
		case string:
			errMsg = msg
		default:
			errMsg = fmt.Sprintf("%v", msg)
		}
	} else {
		status = http.StatusInternalServerError
		errMsg = err.Error()
	}

	// HEAD请求不需要返回响应
	if c.Request.Method == http.MethodHead {
		c.NoContent(status)
		return
	}

	// 返回错误响应
	c.JSON(status, Map{"error": errMsg})
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

func (e *HTTPError) Unwrap() error {
	return e.Internal
}
