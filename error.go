package zest

import (
	"errors"
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

	// 如果是 HTTPError，使用其状态码和消息
	var he *HTTPError
	if errors.As(err, &he) {
		// 处理嵌套的HTTPError
		if he.Internal != nil {
			var herr *HTTPError
			if errors.As(he.Internal, &herr) {
				he = herr
			}
		}
	} else {
		he = &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Msg:        err.Error(),
		}
	}

	// HEAD请求不需要返回响应
	if c.Request.Method == http.MethodHead {
		c.NoContent(he.StatusCode)
		return
	}

	// 返回错误响应
	c.JSON(he.StatusCode, Map{"error": he.Msg})
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
