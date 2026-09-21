# Core 与生命周期分离

## 何时使用

默认可以由 `*Core` 持有后台任务及生命周期方法。只有后台任务的启动、停止和业务调用已经难以独立管理，或依赖图需要明确的生命周期边界时，才考虑独立 Handler。不要因为出现 goroutine、ctx、cancel 或 ticker 字段就要求重构。

Wire 的值类型和指针类型匹配问题与依赖环是两类问题。先查看 provider 的输入、输出和实际依赖链：类型不匹配应统一类型；真正的环需要调整依赖方向或拆开构造与启动。仅把 `*Core` 换成 `Core` 不能保证消除依赖环。

## 职责划分

| 组件 | 职责 |
|------|------|
| Core | 业务查询、计算和状态变更 |
| Handler | 管理后台任务的 context、取消、退出等待与资源释放 |
| 组合入口 | 构造完整依赖图，完成注入后启动任务，关闭时按依赖的逆序清理 |

如果 Handler 只服务一个 Core，可作为 Core 的内部实现，由 Core 转发生命周期操作。如果组合入口需要统一管理多个任务，可以直接持有生命周期接口，不必为了隐藏 Handler 强制所有调用都绕经 Core。

## 构造和启动

- 先完成依赖注入和状态初始化，再启动 goroutine，避免后台任务读取尚未初始化的字段。
- 沿用项目已采用的 Core 值类型或指针类型；不要仅为套用本模式改动公开构造函数。
- 如果 Handler 持有 Core 值副本，确认复制的字段适合共享：接口复制不会复制底层对象，但也不会自动保证线程安全；锁、原子状态和需要同步修改的值不能随意复制。
- 若后台任务必须延后启动，显式区分构造和启动，并由组合入口负责调用，不使用全局变量桥接依赖。

## 关闭与错误处理

以下片段依赖 `context`，适用于单个、只启动一次的后台循环。组合入口完成注入后传入非 nil 的 `run`；`run` 必须响应取消并在正常退出时返回 nil，其他错误原样返回。`Close` 可并发、重复调用，所有调用都等待同一次退出并取得结果。

```go
type Handler struct {
    cancel context.CancelFunc
    done   chan struct{}
    err    error
}

// StartHandler 在依赖就绪后启动循环，并用 done 同步退出结果。
func StartHandler(parent context.Context, run func(context.Context) error) *Handler {
    ctx, cancel := context.WithCancel(parent)
    h := &Handler{cancel: cancel, done: make(chan struct{})}
    go func() {
        defer close(h.done)
        h.err = run(ctx)
    }()
    return h
}

// Close 等待退出，避免清理资源时后台任务仍在使用资源。
func (h *Handler) Close() error {
    h.cancel()
    <-h.done
    return h.err
}
```

若循环另有任务入口，关闭时先阻止新任务进入。最终持久化的位置由状态所有权决定，确保不会与后台写入并发，不一律规定“先持久化再取消”。

最终落库需要仍然有效的 context 和数据库连接。持久化及清理失败应返回给调用方；若清理接口不能返回 error，则使用项目日志记录，不能静默忽略。Wire 的清理函数应覆盖实际资源释放，不能仅取消 context 就认定退出完成。

## 验证

针对实际修改的生命周期验证：依赖构造完成后才启动、启动失败释放已有资源、关闭等待任务退出、重复关闭不阻塞、并发访问共享状态无竞态。检查生成的 Wire 代码和依赖图，不以值类型替换作为依赖环已消除的证据。
