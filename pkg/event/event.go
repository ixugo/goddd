// Package event 提供类型安全的事件通知机制（观察者模式）。
// 每个事件类型对应一个独立的 Bus 实例，无需 map 路由。
// handler 内部可自由选择同步处理或调用 River 入队。
// map 存储天然无序，调用方不应依赖 handler 执行顺序。
package event

import "context"

// Notifier 事件通知器接口（发布方持有，便于测试 mock）。
type Notifier[T any] interface {
	Notify(ctx context.Context, data T) error
}

// Bus 内存同步事件广播器。
// 每种事件类型一个 Bus 实例，观察者通过 Register(key, fn) 注册。
// key 仅用于注册/注销标识，不参与通知路由。
type Bus[T any] struct {
	handlers map[string]func(context.Context, T) error
}

// NewBus 创建事件总线实例。
func NewBus[T any]() *Bus[T] {
	return &Bus[T]{
		handlers: make(map[string]func(context.Context, T) error),
	}
}

// Register 注册观察者函数。key 重复则 panic，防止静默覆盖。
func (b *Bus[T]) Register(key string, fn func(context.Context, T) error) {
	if _, exists := b.handlers[key]; exists {
		panic("event.Bus: duplicate key: " + key)
	}
	b.handlers[key] = fn
}

// Unregister 移除指定 key 的观察者。
func (b *Bus[T]) Unregister(key string) {
	delete(b.handlers, key)
}

// Notify 通知所有已注册的观察者。任一返回 err 则中止并上抛。
// 执行顺序不确定（map 遍历），调用方不应依赖顺序。
func (b *Bus[T]) Notify(ctx context.Context, data T) error {
	for _, fn := range b.handlers {
		if err := fn(ctx, data); err != nil {
			return err
		}
	}
	return nil
}
