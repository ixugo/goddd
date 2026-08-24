package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// buildStackLogger 复刻 SetupSlog 的包装链: slog -> Slog 事件分发 -> zapslog -> zapcore,
// 用于验证 error 级别日志的 stacktrace 字段首帧是否为真实调用点
func buildStackLogger(buf *bytes.Buffer, opts ...zapslog.HandlerOption) *slog.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(buf),
		zapcore.DebugLevel,
	)
	return slog.New(New(zapslog.NewHandler(core, opts...)))
}

// firstStackFrame 解析 JSON 日志中的 stacktrace 字段, 取首个 "函数名\n文件:行" 帧
func firstStackFrame(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()
	var entry struct {
		Stack string `json:"stacktrace"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("解析日志失败: %v, 原始内容: %s", err, buf.String())
	}
	if entry.Stack == "" {
		t.Fatal("error 级别日志缺少 stacktrace 字段")
	}
	return entry.Stack[:strings.Index(entry.Stack, "\n")]
}

func TestStacktrace首帧为真实调用点(t *testing.T) {
	var buf bytes.Buffer
	buildStackLogger(&buf, zapslog.WithCallerSkip(1)).ErrorContext(context.Background(), "boom")

	if frame := firstStackFrame(t, &buf); strings.Contains(frame, "log/slog") {
		t.Fatalf("stacktrace 首帧落入 slog 内部: %s", frame)
	}
}
