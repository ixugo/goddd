# 事件通知（观察者模式）

`pkg/event` 提供类型安全的泛型事件广播，零序列化、天然无序。

## 核心 API

```go
// 创建总线
bus := event.NewBus[UserDeletedEvent]()

// 注册观察者（key 仅标识，不参与路由）
bus.Register("home", homeBus.HandleUserDeleted)

// 注销
bus.Unregister("home")

// 通知（map 遍历，无序）
if err := bus.Notify(ctx, UserDeletedEvent{UserID: 42}); err != nil {
    return err
}
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

Bus 不依赖 River。需要持久化异步任务时，在订阅处理函数中调用项目已有队列适配器，并把入队错误返回给 Bus。River 的调用参数与返回值以项目锁定版本为准，本仓库没有 River 依赖，不应直接复制未经核实的调用示例或仅为使用 Bus 引入它。

同步处理函数与入队处理函数可以共存。业务提交后再入队存在进程中断导致漏发的窗口；要求业务写入与任务持久化原子一致时，使用项目已有的同事务入队或 outbox 方案。

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

- **WithTx**：让兼容的 Store 使用同一数据库事务；原子性取决于各操作实际使用该事务并正确提交或回滚，不覆盖外部服务与通知
- **事件通知**：操作完成后广播副作用（清理、缓存失效、审计）

组合使用时遵循以下顺序：

1. 检查事务开启错误，失败时直接返回。
2. 在同一事务内执行各项 Store 操作，逐项检查错误；失败时回滚，保留原始错误，并记录或合并回滚错误。
3. 检查提交错误，提交失败时不发送成功事件。
4. 提交成功且通知器已配置时调用 `Notify`，按业务约定处理通知错误。

提交后的通知失败不会撤销数据库操作。直接把通知错误返回调用方会出现“请求失败但数据已提交”的结果，应按现有接口契约决定反馈和补偿方式；重试副作用需要幂等。Bus 不持久化事件，也不保证所有观察者都执行成功。
