package web

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ixugo/goddd/pkg/logger"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	maxLen := len(b)
	if maxLen > 100 {
		maxLen = 100
	}
	w.body.Write(b[:maxLen])
	return w.ResponseWriter.Write(b)
}

// Logger 第二个参数是否记录 请求与响应的 body。
func Logger(log *slog.Logger, recordBodyFn func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := uuid.NewString()
		c.Request = c.Request.WithContext(logger.WithAttr(c.Request.Context(), slog.String("trace_id", traceID)))

		var reqBody string
		var blw bodyLogWriter

		recordBody := recordBodyFn != nil && recordBodyFn(c)

		if recordBody {
			// 请求参数
			raw, err := c.GetRawData()
			if err != nil {
				slog.ErrorContext(c.Request.Context(), "logger", "err", err)
			}
			maxL := len(raw)
			if maxL > 100 {
				maxL = 100
			}
			reqBody = string(raw[:maxL])

			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			// 响应参数
			blw = bodyLogWriter{
				body:           bytes.NewBuffer(nil),
				ResponseWriter: c.Writer,
			}
			c.Writer = &blw
		}

		now := time.Now()

		SetTraceID(c, traceID)
		c.Next()

		code := c.Writer.Status()
		out := []any{
			"uid", uid,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"remoteaddr", c.ClientIP(),
			"statuscode", code,
			"since", time.Since(now).Milliseconds(),
		}
		if recordBody {
			out = append(out, []any{"request_body", reqBody, "response_body", blw.body.String()}...)
		}
		if code >= 200 && code < 400 {
			log.InfoContext(c.Request.Context(), "OK", out...)
			return
		}
		// 约定: 返回给客户端的错误，记录的 key 为 responseErr
		errStr, _ := c.Get("responseErr")
		if !(code == 404 || code == 401) {
			out = append(out, []any{"err", errStr})
		}
		log.WarnContext(c.Request.Context(), "Bad", out...)
	}
}
