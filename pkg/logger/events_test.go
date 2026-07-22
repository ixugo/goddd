package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/zap/exp/zapslog"
)

// newTestLogger 创建写内存缓冲的 slog.Logger,便于断言事件回调与日志内容
func newTestLogger(buf *bytes.Buffer, opts ...Option) *slog.Logger {
	core := NewJSONLogger(false, buf, Sampler{}.ensureNonZero()).Core()
	return slog.New(New(zapslog.NewHandler(core), opts...))
}

// TestEvents 事件回调机制
func TestEvents(t *testing.T) {
	orig := Level.Level()
	defer Level.SetLevel(orig)
	SetLevel("debug")

	// 四个级别各自触发对应回调,且回调收到的消息与日志一致
	t.Run("按级别触发", func(t *testing.T) {
		var buf bytes.Buffer
		fired := make(map[slog.Level]string)
		capture := func(level slog.Level) EventFn {
			return func(_ context.Context, r slog.Record) {
				fired[level] = r.Message
			}
		}
		log := newTestLogger(&buf, WithEvents(Events{
			OnDebug: capture(slog.LevelDebug),
			OnInfo:  capture(slog.LevelInfo),
			OnWarn:  capture(slog.LevelWarn),
			OnError: capture(slog.LevelError),
		}))

		ctx := context.Background()
		log.DebugContext(ctx, "调试消息")
		log.InfoContext(ctx, "普通消息")
		log.WarnContext(ctx, "警告消息")
		log.ErrorContext(ctx, "错误消息")

		for level, want := range map[slog.Level]string{
			slog.LevelDebug: "调试消息",
			slog.LevelInfo:  "普通消息",
			slog.LevelWarn:  "警告消息",
			slog.LevelError: "错误消息",
		} {
			if got, ok := fired[level]; !ok || got != want {
				t.Errorf("级别 %v 回调异常: fired=%v, want %q", level, ok, want)
			}
		}
	})

	// 未注册任何回调时,打日志不得 panic
	t.Run("空事件不panic", func(t *testing.T) {
		var buf bytes.Buffer
		log := newTestLogger(&buf)
		ctx := context.Background()
		log.DebugContext(ctx, "a")
		log.InfoContext(ctx, "b")
		log.WarnContext(ctx, "c")
		log.ErrorContext(ctx, "d")
	})

	// 只注册部分级别时,未注册级别不得误触发
	t.Run("未注册级别不触发", func(t *testing.T) {
		var buf bytes.Buffer
		var errorFired bool
		log := newTestLogger(&buf, WithEvents(Events{
			OnError: func(context.Context, slog.Record) { errorFired = true },
		}))
		log.InfoContext(context.Background(), "普通消息")
		if errorFired {
			t.Error("Info 日志误触发 OnError")
		}
		if !bytes.Contains(buf.Bytes(), []byte("普通消息")) {
			t.Error("Info 日志未写入")
		}
		log.ErrorContext(context.Background(), "错误消息")
		if !errorFired {
			t.Error("OnError 未触发")
		}
	})

	// log.With() 派生的 logger 必须保留事件回调
	t.Run("With后事件保留", func(t *testing.T) {
		var buf bytes.Buffer
		var got string
		log := newTestLogger(&buf, WithEvents(Events{
			OnError: func(_ context.Context, r slog.Record) { got = r.Message },
		})).With("service_id", "svc-1")

		log.ErrorContext(context.Background(), "派生错误")
		if got != "派生错误" {
			t.Errorf("With 派生后 OnError 异常: got %q", got)
		}
	})

	// 回调应能读到 WithAttrs 注入的上下文参数
	t.Run("回调可见上下文参数", func(t *testing.T) {
		var buf bytes.Buffer
		var traceID string
		log := newTestLogger(&buf, WithEvents(Events{
			OnInfo: func(_ context.Context, r slog.Record) {
				r.Attrs(func(a slog.Attr) bool {
					if a.Key == "trace_id" {
						traceID = a.Value.String()
					}
					return true
				})
			},
		}))

		ctx := WithAttrs(context.Background(), slog.String("trace_id", "tid-123"))
		log.InfoContext(ctx, "带追踪")
		if traceID != "tid-123" {
			t.Errorf("回调未读到 trace_id: got %q", traceID)
		}
	})

	// 一次调用挂载多个字段,日志应全部带出
	t.Run("批量挂载", func(t *testing.T) {
		var buf bytes.Buffer
		log := newTestLogger(&buf)
		ctx := WithAttrs(context.Background(),
			slog.String("trace_id", "tid-1"),
			slog.String("user_id", "u-1"),
		)
		log.InfoContext(ctx, "批量")
		out := buf.String()
		for _, want := range []string{`"trace_id":"tid-1"`, `"user_id":"u-1"`} {
			if !strings.Contains(out, want) {
				t.Errorf("日志缺少 %s: %s", want, out)
			}
		}
	})
}
