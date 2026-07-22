package logger

import (
	"context"
	"log/slog"
)

// EventFn 日志事件回调,在对应级别的日志写入前同步触发;回调内做耗时操作(如发告警)需自行异步化,否则会拖慢日志写入
// 回调若需异步留存 record,必须先调用 record.Clone(),否则与后续日志共享底层数据存在竞态
type EventFn func(ctx context.Context, r slog.Record)

// Events 按日志级别注册事件回调,未注册的级别零开销
type Events struct {
	OnError EventFn
	OnWarn  EventFn
	OnInfo  EventFn
	OnDebug EventFn
}
