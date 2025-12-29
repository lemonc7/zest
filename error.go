package engx

import (
	"fmt"
	"net/http"
)



func DefaultErrHandlerFunc(err error, c *Context) {
	if he, ok := err.(*HTTPError); ok {
		c.JSON(he.StatusCode, Map{"error": he.Msg})
		return
	}
	c.JSON(http.StatusInternalServerError, Map{"error": err.Error()})
}


func NewHTTPError(statusCode int, msg string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Msg:        msg,
	}
}

func (e *HTTPError) Error() string {
	if e.Internal == nil {
		return fmt.Sprintf("code=%d, message=%s", e.StatusCode, e.Msg)
	}
	return fmt.Sprintf("code=%d, message=%s, error=%v", e.StatusCode, e.Msg, e.Internal)
}

func (e *HTTPError) SetInternal(err error) *HTTPError {
	e.Internal = err
	return e
}
