package engx

import (
	"github.com/Oudwins/zog"
	"github.com/Oudwins/zog/parsers/zjson"
	"github.com/Oudwins/zog/zhttp"
)

// BindBody 使用 zog schema 解析 JSON 请求体
func (c *Context) BindBody(schema *zog.StructSchema, dstPtr any) zog.ZogIssueList {
	return schema.Parse(zjson.Decode(c.Request.Body), dstPtr)
}

// BindQuery 使用 zog schema 解析 Query 参数
func (c *Context) BindQuery(schema *zog.StructSchema, dstPtr any) zog.ZogIssueList {
	return schema.Parse(zhttp.Request(c.Request), dstPtr)
}
