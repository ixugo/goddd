# 事件通知（观察者模式）

`pkg/event` 提供类型安全的泛型事件广播，零序列化、天然无序。

## 核心 API

```go
// 创建总线
bus := event.NewBus[UserDeletedEvent]()

// 注册观察者（key 仅标识，不参与路由）
bus.Register("home", homeBus.HandleUserDeleted)
bus.Register("river:audit", func(ctx context.Context, e UserDeletedEvent) error {
    return riverClient.Insert(ctx, AuditArgs{UserID: e.UserID})
})

// 注销
bus.Unregister("river:audit")

// 通知（map 遍历，无序）
bus.Notify(ctx, UserDeletedEvent{UserID: 42})
```

## Core 集成

```go
type UserDeletedEvent struct { UserID int }

type Core struct {
    store     Storer
    onDeleted event.Notifier[UserDeletedEvent]  // 接口，便于测试 mock
}

func WithOnDeleted(n event.Notifier[UserDeletedEvent]) Option {
    return func(c *Core) { c.onDeleted = n }
}

func (c Core) DeleteUser(ctx context.Context, id int) error {
    if err := c.store.User().Delete(ctx, &User{ID: id}); err != nil {
        return err
    }
    if c.onDeleted != nil {
        return c.onDeleted.Notify(ctx, UserDeletedEvent{UserID: id})
    }
    return nil
}
```

## Wire 装配

```go
// 创建事件总线
userDeletedBus := event.NewBus[user.UserDeletedEvent]()

// 注册观察者
userDeletedBus.Register("home", homeBus.HandleUserDeleted)
userDeletedBus.Register("product", productBus.HandleUserDeleted)

// 注入 Core
userCore := user.NewCore(store, user.WithOnDeleted(userDeletedBus))
```

## River 集成（持久化异步）

handler 函数内部调用 River，Bus 本身不感知：

```go
bus.Register("river:cleanup", func(ctx context.Context, e user.UserDeletedEvent) error {
    _, err := riverClient.Insert(ctx, CleanupUserHomesArgs{UserID: e.UserID})
    return err
})
```

同步 handler 和 River handler 可共存于同一 Bus。

## 设计要点

| 规则 | 说明 |
|------|------|
| map 存储 | 天然无序，调用方不依赖执行顺序 |
| key 仅标识 | Register/Unregister 用 key 管理，不参与路由 |
| 中止上抛 | 任一 handler 返回 err 则中止（与 service Delegate 一致） |
| 泛型类型安全 | 每种事件一个 `Bus[T]`，零序列化 |
| 接口可替换 | `Notifier[T]` 接口便于测试 mock |
| 注册时机 | 启动期完成，运行期只读 |

## 与 WithTx 事务的关系

- **WithTx**：保证多个 Store 操作原子性（全成或全败）
- **事件通知**：操作完成后广播副作用（清理、缓存失效、审计）

两者互补，不互斥。典型组合：

```go
func (c Core) DeleteUser(ctx context.Context, id int) error {
    tx, _ := c.store.Begin()
    defer tx.Rollback()

    txUser, _ := c.store.User().WithTx(tx)
    txUser.Delete(ctx, &User{ID: id})

    if err := tx.Commit(); err != nil {
        return err
    }
    // 事务成功后再触发事件通知
    return c.onDeleted.Notify(ctx, UserDeletedEvent{UserID: id})
}
```
