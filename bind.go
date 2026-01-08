package engx

import (
	"net/http"
	"strings"

	"github.com/Oudwins/zog"
	"github.com/Oudwins/zog/parsers/zjson"
	"github.com/Oudwins/zog/zhttp"
)

// Bind 自动判定请求类型并解析参数
func (c *Context) Bind(schema *zog.StructSchema, dstPtr any) error {
	if c.Request.Method == http.MethodGet {
		return c.BindQuery(schema, dstPtr)
	}

	contentType := c.Request.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return c.BindBody(schema, dstPtr)
	}

	// 兜底尝试 Query 参数（例如 DELETE 请求带 Query）
	return c.BindQuery(schema, dstPtr)
}

// BindBody 使用 zog schema 解析 JSON 请求体
func (c *Context) BindBody(schema *zog.StructSchema, dstPtr any) error {
	errs := schema.Parse(zjson.Decode(c.Request.Body), dstPtr)
	if errs != nil {
		return NewHTTPError(http.StatusBadRequest, zog.Issues.Flatten(errs))
	}
	return nil
}

// BindQuery 使用 zog schema 解析 Query 参数
func (c *Context) BindQuery(schema *zog.StructSchema, dstPtr any) error {
	errs := schema.Parse(zhttp.Request(c.Request), dstPtr)
	if errs != nil {
		return NewHTTPError(http.StatusBadRequest, zog.Issues.Flatten(errs))
	}
	return nil
}
