package logger

import (
	"context"
	"log/slog"
	"slices"
)

const slogFields = "slog_context_fields"

var _ slog.Handler = (*Slog)(nil)

// Slog 包装任意 slog.Handler,在写日志前按级别分发事件回调
// 支持 WithAttrs 批量把字段挂到上下文,后续用该上下文打日志时自动带出这些字段
type Slog struct {
	handler slog.Handler
	events  Events
}

// New 创建事件分发 Handler,事件回调通过 Option 注入,未注入时零回调开销
func New(handler slog.Handler, opts ...Option) *Slog {
	s := Slog{
		handler: handler,
	}
	for _, opt := range opts {
		opt(&s)
	}
	return &s
}

// Handle 先按级别分发事件回调,再交给底层 handler 写日志;事件在底层 handler 之前触发,被采样丢弃的日志同样会触发回调
func (s *Slog) Handle(ctx context.Context, record slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		record.AddAttrs(attrs...)
	}
	switch record.Level {
	case slog.LevelDebug:
		if s.events.OnDebug != nil {
			s.events.OnDebug(ctx, record)
		}
	case slog.LevelInfo:
		if s.events.OnInfo != nil {
			s.events.OnInfo(ctx, record)
		}
	case slog.LevelWarn:
		if s.events.OnWarn != nil {
			s.events.OnWarn(ctx, record)
		}
	case slog.LevelError:
		if s.events.OnError != nil {
			s.events.OnError(ctx, record)
		}
	}
	return s.handler.Handle(ctx, record)
}

// WithAttrs 自行包装底层 handler,保证 log.With() 之后事件回调仍然生效
func (s *Slog) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Slog{handler: s.handler.WithAttrs(attrs), events: s.events}
}

// WithGroup 包装原因同 WithAttrs
func (s *Slog) WithGroup(name string) slog.Handler {
	return &Slog{handler: s.handler.WithGroup(name), events: s.events}
}

// Enabled 委托底层 handler 判断级别,保证事件分发与实际写日志的级别过滤一致
func (s *Slog) Enabled(ctx context.Context, level slog.Level) bool {
	return s.handler.Enabled(ctx, level)
}

// WithAttrs 批量把字段挂到上下文,后续用该上下文打日志时自动带出这些字段
func WithAttrs(parent context.Context, attrs ...slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	old, _ := parent.Value(slogFields).([]slog.Attr)
	v := slices.Concat(old, attrs)
	return context.WithValue(parent, slogFields, v) // nolint
}
