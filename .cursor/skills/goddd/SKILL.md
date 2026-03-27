---
name: goddd
description: GoDDD 六边形架构开发指南。当使用 goddd 架构实现代码、创建新领域、新增 CRUD、数据库表定义、领域间依赖解耦、排序功能、Core 层需要 HTTP 请求信息时使用此技能。也应在以下隐含场景主动触发：新增业务模块、讨论 Core/Store/API 分层、使用 godddx 生成代码、实现适配器模式、添加 Wire provider、使用 web.WrapH/PagerFilter/DateFilter/WithContext 等框架工具。即使用户没有提到"goddd"，只要涉及六边形架构、领域驱动、依赖倒置、CRUD 生成等概念，都应使用此技能。
---

# GoDDD 六边形架构开发指南

本技能指导在 GoDDD 六边形架构下进行开发，覆盖代码生成、领域间解耦、排序方案、HTTP 上下文透传、Web 工具使用等核心场景。

> **遇到不确定的写法时，优先参考项目中已有的符合规范的领域代码。**

## 目录

1. [架构概览](#架构概览)
2. [godddx 代码生成](#godddx-代码生成)
3. [参数定义规范](#参数定义规范)
4. [领域间解耦（适配器模式）](#领域间解耦)
5. [排序功能实现](#排序功能实现)
6. [WithContext：Core 层获取 HTTP 信息](#withcontext)
7. [Web 工具函数速查](#web-工具函数速查)
8. [API 层规范](#api-层规范)

详细参考文档在 `references/` 目录下，按需阅读：
- `references/sort.md` — 排序功能完整实现方案
- `references/with-context.md` — WithContext 架构设计详解
- `references/adapter-pattern.md` — 适配器模式与 Option 注入完整示例
- `references/web-toolkit.md` — Web 工具函数完整参考（含签名和用法示例）

---

## 架构概览

```
┌──────────────────────────────────────────────────────────┐
│                   API 层 (主动适配器)                      │
│  internal/web/api/                                       │
│  职责: HTTP 协议转换 → 调用 Core → 返回响应                │
└──────────────────────┬───────────────────────────────────┘
                       │ 依赖
                       ▼
┌──────────────────────────────────────────────────────────┐
│               Core 层 (领域层/业务核心)                    │
│  internal/core/<domain>/                                 │
│                                                          │
│  ├─ core.go            Core 结构体 + Storer 接口          │
│  ├─ port.go            被动适配器接口                      │
│  ├─ doc.go             领域说明                           │
│  ├─ model.go           非 GORM 类型定义                   │
│  ├─ <entity>.go        业务方法 + EntityStorer 接口        │
│  ├─ <entity>.model.go  领域模型 (GORM 映射)               │
│  ├─ <entity>.param.go  Find/Add/Edit Input 参数           │
│  ├─ <provider>adapter/ 对外提供的适配器实现                 │
│  └─ store/<domain>db/  数据库实现 (被动适配器)             │
└──────────────────────────────────────────────────────────┘
```

**依赖方向**：API → Core ← Store/Adapter（外层依赖内层，内层通过接口反转依赖）

---

## godddx 代码生成

CRUD 场景**必须**使用 [godddx](https://github.com/ixugo/godddx) 生成代码，确保结构一致。

### 步骤

1. 在 `tables/<domain>/` 下创建表定义文件
2. 结构体**必须包含** `ID`、`CreatedAt`、`UpdatedAt` 字段
3. 若使用随机字符 ID，使用 `uniqueid.Core` 类型
4. 同一领域多个结构体放在同一个 tables 文件中
5. 执行生成：`godddx -f tables/<domain>/<entity>.go`
6. 在 `internal/web/api/provider.go` 注册 Wire provider
7. 调用生成的 `Register<Domain>` 函数注册路由
8. 在领域目录下创建 `doc.go` 描述领域用途

### 表定义示例

```go
// tables/task/task.go
package task

import (
    "time"
    "github.com/ixugo/goddd/domain/uniqueid"
)

type Task struct {
    ID        uniqueid.Core `gorm:"primaryKey"`
    Name      string
    Status    int
    TenantID  string
    CreatedBy string
    Sort      int64  `gorm:"autoIncrement"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Wire 注册模式

在 `provider.go` 中添加 `NewXxxCore` 和 `NewXxxAPI`：

```go
var ProviderSet = wire.NewSet(
    wire.Struct(new(Usecase), "*"),
    NewHTTPHandler,
    // ... 已有 provider
    NewTaskCore, NewTaskAPI,  // 新增领域
)

func NewTaskCore(db *gorm.DB) task.Core {
    store := taskdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return task.NewCore(store)
}
```

---

## 参数定义规范

**核心原则**：归属字段（TenantID、CreatedBy）由 API 层填充，编辑时不可修改。

### FindInput — 查询参数

```go
type FindEntityInput struct {
    web.PagerFilter                          // 分页
    web.DateFilter                           // 日期范围（start_ms, end_ms 毫秒时间戳）
    Name string `form:"name"`               // 模糊查询字段

    TenantID  string `form:"-"`             // API 层填充
    CreatedBy string `form:"-"`             // API 层填充
}
```

### AddInput — 新增参数

```go
type AddEntityInput struct {
    Name string `json:"name"`

    TenantID  string `json:"-"`             // API 层填充
    CreatedBy string `json:"-"`             // API 层填充
}
```

### EditInput — 编辑参数

```go
type EditEntityInput struct {
    Name string `json:"name"`
    // 不包含 TenantID、CreatedBy 等归属字段
}
```

---

## 领域间解耦

领域间**必须**通过适配器解耦，不能直接依赖其他领域的 Core。

### 核心规则

| 规则 | 说明 |
|------|------|
| Port 定义在**提供方** | 接口和模型定义在提供能力的领域子包中 |
| Adapter 实现在**提供方** | 适配器放在 `<provider>adapter/` 子包 |
| 消费方通过 **Option 注入** | `NewCore(store, opts...)` 模式 |
| 返回类型定义在**提供方子包** | 避免重复定义 |

### Option 注入模式

```go
// 消费方 core.go
type Core struct {
    store        Storer
    userProvider useradapter.BriefProvider
}

type Option func(*Core)

func WithUserProvider(p useradapter.BriefProvider) Option {
    return func(c *Core) { c.userProvider = p }
}

func NewCore(store Storer, opts ...Option) Core {
    c := Core{store: store}
    for _, opt := range opts { opt(&c) }
    return c
}
```

### API 层注入

```go
func NewMessageCore(db *gorm.DB, briefProvider useradapter.BriefProvider) message.Core {
    store := messagedb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return message.NewCore(store,
        message.WithUserProvider(briefProvider),
    )
}
```

> 详细示例和完整代码请阅读 `references/adapter-pattern.md`

---

## 排序功能实现

实现拖拽排序：接收有序 ID 数组，重新分配 sort 值而不影响未传入的记录。

### 核心逻辑

1. 查询传入 ID 的记录，获取现有 `sort` 值
2. 将 `sort` 值升序排列
3. 按传入 ID 顺序重新分配排序值
4. 事务批量更新

> 完整实现代码请阅读 `references/sort.md`

### 要点

- 数据库字段 `sort` 使用 gorm tag `autoIncrement` 自增
- Store 层用事务批量更新，Core 层编排逻辑，API 层只做协议转换
- 校验所有 ID 存在，不存在则返回错误

---

## WithContext

解决 Core 层不依赖 HTTP 框架、但 Adapter 需要 HTTP 请求信息的矛盾。

### 原理

`web.Context` 扩展了 `context.Context`，携带 `*http.Request`，通过类型断言按需获取：

```go
// API 层 — 构造 web.Context
ctx := web.WithContext(c.Request)
core.DoSomething(ctx, ...)

// Adapter 层 — 类型断言获取 HTTP 信息
func (p *impl) resolveCover(ctx context.Context, cover string) string {
    if wc, ok := ctx.(web.Context); ok {
        return p.coverURLFunc(wc.Request(), cover)
    }
    return cover  // 非 HTTP 场景降级
}
```

### 设计要点

| 特性 | 说明 |
|------|------|
| 零破坏性 | 实现 `context.Context`，现有签名无需修改 |
| 渐进式采用 | 只改调用处（API）和使用处（Adapter），Core 层透传 |
| 优雅降级 | 类型断言失败时返回原始值，非 HTTP 场景正常工作 |
| 可扩展 | 接口可定义子接口扩展 |

### 适用场景

- 动态 URL 拼接（需要 scheme + host）
- 请求级元信息透传（IP、User-Agent）
- 跨领域 Adapter 需要 HTTP 上下文辅助数据转换

### 不适用

- 已有明确参数的简单场景 → 直接传参
- 与 HTTP 无关的逻辑 → 使用标准 `context.Context`
- 需要修改请求状态 → 在 API 层处理

> 完整设计文档请阅读 `references/with-context.md`

---

## Web 工具函数速查

`github.com/ixugo/goddd/pkg/web` 提供 HTTP 开发基础设施。

> 完整函数签名和使用示例请阅读 `references/web-toolkit.md`

### 请求处理

| 函数/类型 | 用途 |
|----------|------|
| `WrapH(fn)` | 核心包装函数，将 `func(*gin.Context, *Input) (Output, error)` 包装为 `gin.HandlerFunc` |
| `WrapHs(fn, mid...)` | 同 WrapH，附加前置中间件 |
| `PagerFilter` | 分页参数（Page, Size, Sort, SortSafelist），含 `Offset()`, `Limit()`, `SortColumn()` |
| `NewPagerFilterMaxSize()` | 不分页查询（Size=99999） |
| `DateFilter` | 日期范围（StartMs, EndMs 毫秒时间戳），含 `StartAt()`, `EndAt()`, `DefaultStartAt()` |
| `Validator` | 参数校验，`Check(ok, key, msg)`, `AddError(key, msg)`, `Valid()`, `List()` |
| `CustomMethods` | 自定义方法路由 |

### 响应处理

| 函数/类型 | 用途 |
|----------|------|
| `PageOutput[T]` | 分页响应 `{Items, Total}` |
| `ScrollPageOutput[T]` | 滚动分页 `{Items, Next}` |
| `Success(c, data)` | 统一成功响应 |
| `Fail(c, err)` | 统一错误响应，自动映射 HTTP 状态码 |
| `ResponseMsg` | 通用消息响应 `{Msg}` |

### Context 与 URL

| 函数/类型 | 用途 |
|----------|------|
| `WithContext(r)` | `*http.Request` → `web.Context`，携带 HTTP 元信息 |
| `GetBaseURL(r)` | 提取 `scheme://host` |
| `BaseURLJoin(r,...string)` | 拼接 `scheme://host/fullpath`|
| `GetHost(r)` / `GetScheme(r)` | 提取 host / scheme |
| `XForwardedPrefix(r, path)` | 处理反向代理前缀 |
| `TraceID(ctx)` / `MustTraceID(ctx)` | 获取请求追踪 ID |
| `SetTraceID(ctx, id)` | 设置追踪 ID |

### JWT 鉴权

| 函数 | 用途 |
|------|------|
| `NewToken(data, secret, opts...)` | 创建 JWT（默认 6h 过期） |
| `ParseToken(token, secret)` | 解析 JWT |
| `AuthMiddleware(secret)` | JWT 鉴权中间件 |
| `AuthLevel(level)` | 等级鉴权中间件（等级越小权限越大） |
| `NewClaimsData()` | 创建 Claims 数据，链式 `SetUserID/SetUsername/SetRoleID/SetLevel/Set` |
| `GetUID/GetUsername/GetRoleID/GetLevel/GetToken/GetInt` | 从上下文获取用户信息 |

### 中间件

| 函数 | 用途 |
|------|------|
| `Logger(ignoreFn...)` | 请求日志 |
| `LoggerWithBody(limit, ignoreFn...)` | 记录请求体/响应体 |
| `LoggerWithUseTime(maxLimit, ignoreFn...)` | 耗时记录，超时打 warn |
| `RateLimiter(r, b)` | 全局限流 |
| `IPRateLimiterForGin(r, b)` | 按 IP 限流 |
| `IDRateLimiter(r, b, ttl)` | 按 ID 限流 |
| `LimitContentLength(limit)` | 请求体大小限制 |
| `CacheControlMaxAge(second)` | Cache-Control 头 |
| `EtagHandler()` | ETag + 304 支持 |
| `Recover()` | panic 恢复 |
| `SetDeadline(duration)` | 读写超时 |
| `Metrics()` | 请求计数统计 |

### SSE（Server-Sent Events）

| 函数/类型 | 用途 |
|----------|------|
| `NewSSE(length, timeout)` | 创建 SSE 实例 |
| `SSE.Publish(event)` | 发布事件 |
| `SSE.ServeHTTP(w, r)` | 实现 http.Handler |
| `SendChunk(ch, c)` / `SendChunkPro(ch, c)` | 分块进度发送 |
| `NewEventMessage(event, data)` | 创建 SSE 事件消息 |

### 忽略选项（用于日志/限流中间件）

| 函数 | 用途 |
|------|------|
| `IgnorePrefix(prefix...)` | 忽略路径前缀 |
| `IgnorePath(path...)` | 忽略完整路径 |
| `IgnoreMethod(method)` | 忽略 HTTP 方法 |
| `IgoreContains(substrs...)` | 忽略路径含子串 |
| `IgnoreBool(v)` | 固定布尔值忽略 |

### 性能分析

| 函数 | 用途 |
|------|------|
| `SetupPProf(r, &ips)` | 注册 pprof 路由，IP 白名单 |
| `SetupMutexProfile(rate)` | 启用互斥锁采样 |
| `CountGoroutines(d, num)` | 记录 goroutine 数量 |

### WrapH 入参规则

- POST/PUT/DELETE/PATCH → 绑定 Request Body（`json` tag）
- GET → 绑定 URL Query（`form` tag）
- 入参第二个参数必须是指针，`*struct{}` 表示无参数
- 路由参数用 `c.Param()` 获取，不走自动绑定

### 错误处理

Core 层返回 `reason.Error` 类型错误，`web.WrapH` 自动映射 HTTP 状态码：

```go
return nil, reason.ErrBadRequest.SetMsg("参数不合法")     // → 400
return nil, reason.ErrDB.Withf("查询失败: %s", err)       // → 500
return nil, reason.ErrUnauthorized.SetMsg("未登录")       // → 401
```

- `SetMsg()` — 给用户的友好提示
- `Withf()` — 给开发者的 details（`SetRelease()` 后不输出）

---

## API 层规范

1. **只做 HTTP 协议转换**：参数绑定 → 填充归属字段 → 调用 Core → 返回响应
2. **归属字段在 API 层填充**：TenantID、CreatedBy 等通过 `json:"-"` / `form:"-"` 标记
3. **路由参数用 `c.Param`**：不走自动绑定，仅路由参数时入参用 `_ *struct{}`
4. **适配器不定义在 API 层**：统一放在领域的 `<provider>adapter/` 目录
5. **Handler 若需访问后续赋值字段**：使用指针接收者

### 路由注册模式

```go
func registerTask(r gin.IRouter, api TaskAPI, handler ...gin.HandlerFunc) {
    g := r.Group("/tasks", handler...)
    g.GET("", web.WrapH(api.findTasks))
    g.POST("", web.WrapH(api.addTask))
    g.GET("/:id", web.WrapH(api.getTask))
    g.PUT("/:id", web.WrapH(api.editTask))
    g.DELETE("/:id", web.WrapH(api.deleteTask))
    g.PUT("/sort", web.WrapH(api.sortTasks))
}
```

### 常用导入

```go
import (
    "github.com/ixugo/goddd/pkg/orm"
    "github.com/ixugo/goddd/pkg/reason"
    "github.com/ixugo/goddd/pkg/web"
    "github.com/ixugo/goddd/domain/uniqueid"
    "github.com/jinzhu/copier"
)
```
