package web

import (
	"bytes" // nolint
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/hook"
)

type EtagWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *EtagWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *EtagWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

// WebCache 主要用于缓存静态资源
// Cache-Control: max-age=3600    # 缓存1小时
// Cache-Control: no-cache        # 每次都需要验证
// Cache-Control: no-store        # 完全不缓存
// Cache-Control: private         # 只允许浏览器缓存
// Cache-Control: public          # 允许中间代理缓存
func CacheControlMaxAge(second int, ignoreFn ...IngoreOption) gin.HandlerFunc {
	age := strconv.Itoa(second)
	return func(ctx *gin.Context) {
		for _, fn := range ignoreFn {
			if fn(ctx) {
				ctx.Next()
				return
			}
		}
		if ctx.Request.Method == "GET" {
			ctx.Header("Cache-Control", "max-age="+age)
		}
		ctx.Next()
	}
}

// EtagHandler 添加 ETag 头，用于缓存静态资源
// 不适合大文件场景，每次都是实时计算的
func EtagHandler(ignoreFn ...IngoreOption) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, fn := range ignoreFn {
			if fn(ctx) {
				ctx.Next()
				return
			}
		}
		bw := EtagWriter{
			ResponseWriter: ctx.Writer,
		}
		ctx.Writer = &bw
		ctx.Next()

		hash, _ := hook.MD5FromIO(&bw.body)
		etag := `"` + hash + `"`
		if match := ctx.GetHeader("If-None-Match"); match != "" && match == etag {
			ctx.Writer.WriteHeader(http.StatusNotModified)
			return
		}
		ctx.Header("ETag", etag)
		if _, err := bw.ResponseWriter.Write(bw.body.Bytes()); err != nil {
			slog.ErrorContext(ctx.Request.Context(), "write err", "err", err)
		}
	}
}
