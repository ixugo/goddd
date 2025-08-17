package web

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ixugo/goddd/pkg/logger"
)

const DefaultBodyLimit = 100

type BufferWriter struct {
	gin.ResponseWriter
	body  bytes.Buffer
	limit int
}

func (w *BufferWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *BufferWriter) Write(b []byte) (int, error) {
	// limit <= 0 时，不限制写入
	if w.limit <= 0 {
		w.body.Write(b)
		return w.ResponseWriter.Write(b)
	}

	remain := w.limit - w.body.Len()
	if remain <= 0 {
		return w.ResponseWriter.Write(b)
	}
	w.body.Write(b[:min(len(b), remain)])
	return w.ResponseWriter.Write(b)
}

// IgnoreBool 忽略指定值
func IgnoreBool(v bool) func(*gin.Context) bool {
	return func(*gin.Context) bool {
		return v
	}
}

// IgnoreMethod 忽略指定请求方式的请求，一般用于忽略 options
func IgnoreMethod(method string) func(*gin.Context) bool {
	return func(ctx *gin.Context) bool {
		return ctx.Request.Method == method
	}
}

// IgnorePrefix 忽略指定路由前缀
func IgnorePrefix(prefix ...string) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		for _, p := range prefix {
			if strings.HasPrefix(c.Request.URL.Path, p) {
				return true
			}
		}
		return false
	}
}

// IgoreContains 忽略包含的路由
func IgoreContains(substrs ...string) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		for _, p := range substrs {
			if strings.Contains(c.Request.URL.Path, p) {
				return true
			}
		}
		return false
	}
}

// Logger 记录 http 请求日志
// 入参是忽略函数，返回 true 则忽略，比如网页请求可以忽略
func Logger(ignoreFn ...func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := uuid.NewString()
		c.Request = c.Request.WithContext(logger.WithAttr(c.Request.Context(), slog.String("trace_id", traceID)))
		SetTraceID(c, traceID)

		for _, fn := range ignoreFn {
			if fn(c) {
				c.Next()
				return
			}
		}

		now := time.Now()
		c.Next()

		code := c.Writer.Status()
		out := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"remoteaddr", c.ClientIP(),
			"statuscode", code,
			"since", time.Since(now).Milliseconds(),
		}
		if code >= 200 && code < 400 {
			slog.InfoContext(c.Request.Context(), "OK", out...)
			return
		}
		// 约定: 返回给客户端的错误，记录的 key 为 responseErr
		errStr, _ := c.Get(ResponseErr)
		if !(code == 404 || code == 401) {
			out = append(out, "err", errStr)
		}
		slog.WarnContext(c.Request.Context(), "Bad", out...)
	}
}

// LoggerWithBody 记录请求体与响应体，通常用于开发调试
// 日志级别是 debug，即没有忽略也可能因为日志级别不打印内容
// limit 用于限制打印数据的大小，防止超大请求体或响应体
func LoggerWithBody(limit int, ignoreFn ...func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, fn := range ignoreFn {
			if fn(c) {
				c.Next()
				return
			}
		}

		// request body
		var reqBody string
		raw, err := c.GetRawData()
		if err == nil {
			l := min(len(raw), limit)
			reqBody = string(raw[:l])
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		// response body
		blw := BufferWriter{
			ResponseWriter: c.Writer,
			limit:          limit,
		}
		c.Writer = &blw
		c.Next()

		if c.Writer.Status() != 404 {
			slog.DebugContext(c.Request.Context(), "body", "req", reqBody, "resp", blw.body.String())
		}
	}
}

// LoggerWithUseTime 记录请求用时
// >= maxLimit 时，记录 warn 级别日志
func LoggerWithUseTime(maxLimit time.Duration, ignoreFn ...func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, fn := range ignoreFn {
			if fn(c) {
				c.Next()
				return
			}
		}

		now := time.Now()
		c.Next()
		since := time.Since(now)

		if since >= maxLimit {
			slog.WarnContext(c.Request.Context(), "check use time", "since", since.Milliseconds())
		}
	}
}
