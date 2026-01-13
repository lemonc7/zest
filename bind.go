package zest

import (
	"fmt"
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
		flattened := zog.Issues.Flatten(errs)
		return NewHTTPError(http.StatusBadRequest, flattenIssues(flattened))
	}
	return nil
}

// BindQuery 使用 zog schema 解析 Query 参数
func (c *Context) BindQuery(schema *zog.StructSchema, dstPtr any) error {
	errs := schema.Parse(zhttp.Request(c.Request), dstPtr)
	if errs != nil {
		flattened := zog.Issues.Flatten(errs)
		return NewHTTPError(http.StatusBadRequest, flattenIssues(flattened))
	}
	return nil
}

func flattenIssues(flattened map[string][]string) string {
	parts := make([]string, 0, len(flattened))
	for field, issues := range flattened {
		if len(issues) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(issues, ", ")))
	}
	return strings.Join(parts, "; ")
}
