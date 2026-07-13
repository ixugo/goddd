package logger

import (
	"context"
	"log/slog"

	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

const slogFields = "slog_context_fields"

type Slog struct {
	*zapslog.Handler
}

func newSlog(core zapcore.Core, opts ...zapslog.HandlerOption) *Slog {
	return &Slog{
		Handler: zapslog.NewHandler(core, opts...),
	}
}

func (s *Slog) Handle(ctx context.Context, record slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		record.AddAttrs(attrs...)
	}
	return s.Handler.Handle(ctx, record)
}

// WithAttr 使用此函数创建的上下文，当应用在 slog 上下文时，会自动记录存在 context 中的参数
func WithAttr(parent context.Context, attr slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attr)
		return context.WithValue(parent, slogFields, v) // nolint
	}
	v := []slog.Attr{attr}
	return context.WithValue(parent, slogFields, v) // nolint
}
