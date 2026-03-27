package web

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

const traceIDKey = "TRACE_ID_KEY"

func MustTraceID(ctx context.Context) string {
	v := ctx.Value(traceIDKey)
	return v.(string)
}

func TraceID(ctx context.Context) (string, bool) {
	v := ctx.Value(traceIDKey)
	if v == nil {
		return "", false
	}
	return v.(string), true
}

func SetTraceID(ctx *gin.Context, id string) {
	ctx.Set(traceIDKey, id)
}

// Context 扩展 context.Context，携带 HTTP 请求的元信息。
// 下游通过 ctx.(web.Context) 类型断言按需使用，不破坏函数签名。
// 断言失败时应优雅降级（返回原始值），保证非 HTTP 场景和单元测试正常工作。
type Context interface {
	context.Context
	Request() *http.Request       // 原始 HTTP 请求
	GetBaseURL() string           // 如 "http://127.0.0.1:8080"
	GetScheme() string            // "http" | "https"
	GetHost() string              // 如 "127.0.0.1"
	BaseURLJoin(...string) string // 拼接 base URL
}

type httpRequestContext struct {
	context.Context
	req *http.Request
}

// WithContext 将 *http.Request 包装为 HTTPRequestContext，
// 使下游可通过类型断言获取 HTTP 请求元信息。
func WithContext(r *http.Request) Context {
	return &httpRequestContext{Context: r.Context(), req: r}
}

func (c *httpRequestContext) Request() *http.Request             { return c.req }
func (c *httpRequestContext) GetBaseURL() string                 { return GetBaseURL(c.req) }
func (c *httpRequestContext) GetScheme() string                  { return GetScheme(c.req) }
func (c *httpRequestContext) GetHost() string                    { return GetHost(c.req) }
func (c *httpRequestContext) BaseURLJoin(paths ...string) string { return BaseURLJoin(c.req, paths...) }
