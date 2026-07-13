# Core 与生命周期分离

> **定位：应急方案，非默认规范。**
> 大多数场景下 `*Core` 指针类型 + 生命周期方法直接挂 Core 即可，可读性更高。
> 仅当 Wire 注入因值类型/指针类型冲突产生循环依赖、或被迫引入全局变量桥接时，才启用本模式。

当领域 Core 需要后台 goroutine（定时持久化、心跳检测、定时清理等），且 Wire 无法解析依赖顺序时，可将生命周期拆到独立 Handler。

**原则：Core 做值类型，生命周期拆到独立的 Handler，Core 内嵌 Handler 指针作为字段。**

## 结构

```go
// Core 业务核心，值类型。生命周期管理委托给内嵌的 SessionHandler。
type Core struct {
    store             Storer
    sessionStore      SessionStore
    viewCountAdjuster ViewCountAdjuster
    maxViewsPerDay    int

    ss *SessionHandler // 生命周期管理器，由 NewCore 创建
}

// SessionHandler 管理 Core 的生命周期：goroutine、ctx、优雅停机。
type SessionHandler struct {
    core   Core // 持有 Core 值副本，用于 goroutine 中调用业务方法
    ctx    context.Context
    cancel context.CancelFunc
    quit   chan struct{}
    once   sync.Once
}
```

## 构造函数

```go
func NewCore(store Storer, sessionStore SessionStore, opts ...Option) (Core, func()) {
    c := Core{
        store:          store,
        sessionStore:   sessionStore,
        maxViewsPerDay: 3,
    }
    for _, opt := range opts {
        opt(&c)
    }
    s := &SessionHandler{core: c}
    s.ctx, s.cancel = context.WithCancel(context.Background())
    s.quit = make(chan struct{})
    go s.persistence()
    c.ss = s
    return c, c.Close
}
```

## 方法分配

| 方法归属 | 判断标准 | 示例 |
|----------|---------|------|
| Core 方法 | 查询 DB、纯业务计算 | GetOverallStats, GetTimeserieStats |
| Core 委托方法 | 需要转发给 SessionHandler | TrackHeartbeat, ActiveViewers, Close |
| SessionHandler 方法 | goroutine 内部调用 | persistence, PersistSessions |

Core 上的委托方法只是一行转发：

```go
func (c Core) TrackHeartbeat(mediaID, sessionID, userID string, currentTimeSec int) {
    c.ss.TrackHeartbeat(mediaID, sessionID, userID, currentTimeSec)
}
func (c Core) Close() { c.ss.Close() }
func (c Core) ActiveViewers(mediaID string) int { return c.ss.ActiveViewers(mediaID) }
```

## Wire 注入

```go
// 返回值类型 Core，不是指针
func NewViewerCore(...) (viewer.Core, func()) { ... }

// API 层持有值类型
type ViewerAPI struct {
    core      viewer.Core
    mediaCore media.Core
}
type ProgressAPI struct {
    progressCore progress.Core
    viewerCore   viewer.Core  // 值类型，通过它调用 TrackHeartbeat / ActiveViewers
}
```

## 适用场景

- 领域 Core 需要后台 goroutine（定时持久化、定时清理、心跳超时检测等）
- Core 被多个上层模块依赖，且依赖方向不同
- Wire 注入出现循环依赖，被迫使用全局变量桥接

## 注意事项

- Core 做值类型时，内部的接口字段（Storer、SessionStore）仍是引用语义，值拷贝安全
- Option 签名为 `func(*Core)`，在 NewCore 中先 `opt(&c)` 再赋值给 SessionHandler
- SessionHandler.Close 必须等待 goroutine 退出（`<-quit`），防止资源泄漏
- Core 的委托方法用值接收者 `(c Core)`，因为 `c.ss` 是指针，转发不影响生命周期

## 优势

1. **无全局变量**：SessionHandler 持有 Core 值副本，不需要全局注册
2. **无 Wire 循环依赖**：Core 是值类型，NewCore 返回值而非指针
3. **职责清晰**：业务逻辑在 Core，生命周期在 SessionHandler
4. **API 统一**：外部只接触 Core（值类型），SessionHandler 是内部实现细节
5. **Close 安全**：先 PersistSessions 落库，再 cancel + 等待 goroutine 退出

## 反模式警告

以下做法会破坏 SRP，遇到时需重构：

**反模式 1：Core 直接持有 goroutine 控制字段**

```go
// ❌ 错误：Core 同时承载业务逻辑和生命周期管理
type Core struct {
    store  Storer
    ctx    context.Context    // 生命周期字段混入业务结构体
    cancel context.CancelFunc
    quit   chan struct{}
    ticker *time.Ticker
}
```

Core 一旦持有 ctx/cancel/ticker，就意味着它既要处理业务又要管理 goroutine 生死，可读性和测试难度都会上升。

**反模式 2：用全局变量桥接两个领域**

```go
// ❌ 错误：用包级变量绕过 Wire 注入顺序问题
var globalViewerCore *viewer.Core

func init() {
    globalViewerCore = &viewer.Core{}
}
```

全局变量掩盖了依赖关系，使初始化顺序不可控，且无法在测试中替换。正确做法是让 Core 保持值类型，由 Wire 的 `func()` 清理函数统一管理生命周期。

**反模式 3：SessionHandler 暴露给外部调用方**

```go
// ❌ 错误：外部直接操作 Handler
type ViewerAPI struct {
    handler *viewer.SessionHandler  // 应该只持有 viewer.Core
}
```

外部调用方只应持有 `Core` 值类型，`SessionHandler` 是 Core 的内部实现细节，不应透出。
