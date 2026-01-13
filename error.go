package zest

import (
	"net/http"
)

type HTTPError struct {
	Code    int
	Message string
	err     error
}

func DefaultErrHandlerFunc(c *Context, err error) {
	// 响应已经提交，直接返回
	if c.Response().Committed {
		return
	}

	var status int
	var errMsg string
	if he, ok := err.(*HTTPError); ok {
		status = he.Code
		errMsg = he.Message
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

func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		Code:    statusCode,
		Message: message,
	}
}

func (e *HTTPError) Error() string {
	if e.Message == "" {
		return http.StatusText(e.Code)
	}
	return e.Message
}

func (e *HTTPError) Wrap(err error) *HTTPError {
	e.err = err
	return e
}

func (e *HTTPError) Unwrap() error {
	return e.err
}
