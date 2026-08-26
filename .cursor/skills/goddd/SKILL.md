---
name: goddd
description: >
  GoDDD 六边形架构开发指南。当使用 goddd 架构实现代码、创建新领域、新增 CRUD、
  数据库表定义、领域间依赖解耦、领域内子包拆分与依赖、排序功能、
  Core 层需要 HTTP 请求信息时使用此技能。
  也应在以下隐含场景主动触发：新增业务模块、讨论 Core/Store/API 分层、
  使用 goddd gen 生成代码、实现适配器模式、添加 Wire provider、
  使用 web.WrapH/PagerFilter/DateFilter/WithContext 等框架工具、
  Core 需要后台任务/定时任务/心跳检测/goroutine、优雅停机、Wire 循环依赖、
  Core 生命周期分离、SessionHandler、
  修改 stores/xxxcache 缓存层（判断内存缓存 vs Redis 缓存、SETNX/SETEX 防竞态、
  WarmUp 预热）、领域内子包间依赖方向、子包是否需要接口隔离、子包循环依赖处理。
  即使用户没有提到"goddd"，只要涉及六边形架构、领域驱动、依赖倒置、CRUD 生成、
  Core 职责过重、缓存层改造、子包拆分依赖等概念，都应使用此技能。
---

# GoDDD 六边形架构开发指南

> **遇到不确定的写法时，优先参考项目中已有的符合规范的领域代码。**

## references 索引

实现对应功能时**必须**先阅读对应文档：

| 文档 | 必读时机 | 核心内容 |
|------|---------|---------|
| `references/api-design-patterns.md` | 设计新接口、审查接口规范 | 资源命名、标准方法、自定义方法、错误处理、分页过滤、校验、限流、路由模板 |
| `references/web-toolkit.md` | 使用 `pkg/web` 任意函数 | WrapH、PagerFilter/DateFilter、JWT、中间件、SSE、Validator、响应封装 |
| `references/adapter-pattern.md` | 新增领域间依赖 | Port/Adapter 定义位置、Option 注入、Wire 装配、port.go/model.go 分工 |
| `references/cache-layer.md` | 修改 `stores/xxxcache/` | 内存 vs Redis、redis.Cmdable、SETNX 防竞态、singleflight、WarmUp、穿透防护 |
| `references/lifecycle-split.md` | Core 需后台 goroutine 且 Wire 循环依赖 | Core 值类型 + SessionHandler、方法分配、`(Core, func())`、反模式 |
| `references/sort.md` | 实现拖拽排序 | 有序 ID 数组 → 收集 sort 值 → 重分配 → 事务更新 |
| `references/with-context.md` | Core/Adapter 需 HTTP 请求信息 | `web.WithContext` → Core 透传 → Adapter 类型断言 |

---

## 函数与类型速查索引

### 请求处理（→ `web-toolkit.md`）

| 函数/类型 | 用途 |
|----------|------|
| `web.WrapH(fn)` | `func(*gin.Context, *Input) (Output, error)` → `gin.HandlerFunc` |
| `web.WrapHs(fn, mid...)` | 同 WrapH，附加前置中间件 |
| `web.PagerFilter` | 分页（Page, Size, Sort），含 `Offset()`, `Limit()`, `SortColumn()` |
| `web.NewPagerFilterMaxSize()` | 不分页全量查询 |
| `web.DateFilter` | 日期范围（StartMs, EndMs），含 `StartAt()`, `EndAt()` |
| `web.Validator` | 业务校验，`Check(ok, key, msg)`, `Valid() bool`, `List() []string` |

### 响应与错误（→ `web-toolkit.md` + `api-design-patterns.md`）

| 函数/类型 | 用途 |
|----------|------|
| `web.Success(c, data)` | 统一成功响应 |
| `web.Fail(c, err)` | 统一错误响应，自动映射状态码 |
| `web.AbortWithStatusJSON(c, err)` | 错误 + Abort（中间件用） |
| `web.PageOutput[T]` | 分页响应 `{Items, Total}` |
| `web.ScrollPageOutput[T]` | 滚动分页 `{Items, Next}` |

### reason.Error 变量（→ `api-design-patterns.md`）

| 变量 | 用途 | 状态码 |
|------|------|-------|
| `ErrBadRequest` | 请求参数有误 | 400 |
| `ErrDB` | 数据库错误 | 400 |
| `ErrNotFound` | 资源未找到 | 400 |
| `ErrServer` | 服务器错误 | 400 |
| `ErrJSON` / `ErrUsedLogic` / `ErrPermissionDenied` / `ErrTimeout` | 其它业务错误 | 400 |
| `ErrFileUpload` / `ErrFileTooLarge` / `ErrContentTooLarge` | 文件相关 | 400 |
| `ErrUnauthorized` | 认证失败 | 401 |
| `ErrTooManyRequests` | 限流 | 429 |

方法：`SetMsg(msg)` 用户提示、`Withf(fmt, ...)` 开发 details、`SetHTTPStatus(code)` 覆盖状态码

### Context 与 URL（→ `web-toolkit.md` + `with-context.md`）

| 函数 | 用途 |
|------|------|
| `web.WithContext(r)` | `*http.Request` → `web.Context` |
| `web.GetBaseURL(r)` / `web.BaseURLJoin(r, paths...)` | 提取/拼接 URL |
| `web.TraceID(ctx)` / `web.MustTraceID(ctx)` | 请求追踪 ID |

### JWT 鉴权（→ `web-toolkit.md`）

| 函数 | 用途 |
|------|------|
| `web.NewToken(data, secret, opts...)` / `web.ParseToken(token, secret)` | 创建/解析 JWT |
| `web.AuthMiddleware(secret)` / `web.AuthLevel(level)` | 鉴权中间件 |
| `web.NewClaimsData()` | Claims 链式构建 |
| `web.GetUID` / `GetUsername` / `GetRoleID` / `GetLevel` / `GetToken` | 获取用户信息 |

### 中间件（→ `web-toolkit.md`）

| 函数 | 用途 |
|------|------|
| `web.Logger` / `LoggerWithBody` / `LoggerWithUseTime` | 请求日志 |
| `web.RateLimiter` / `IPRateLimiterForGin` / `IDRateLimiter` | 限流 |
| `web.Recover()` / `web.SetDeadline(d)` / `web.Metrics()` | 恢复/超时/指标 |

### SSE（→ `web-toolkit.md`）

| 函数/类型 | 用途 |
|----------|------|
| `web.NewSSE` / `SSE.Publish` / `SSE.Close` / `SSE.Stop` | SSE 生命周期 |
| `web.SendChunk` / `SendChunkPro` / `SendSSE` | 流式发送 |
| `web.NewEventMessage(event, data)` | 创建事件消息 |

### 缓存操作（→ `cache-layer.md`）

| 操作 | 命令 | 理由 |
|------|------|------|
| 读穿透回填 | `singleflight.Do` + `SetNX` | 合并并发，不覆盖新值 |
| Create | 不写缓存 | 等读时 SetNX 回填 |
| Update | `Set(key, val, ttl)` | 最新值覆盖 |
| WarmUp | `SetNX` | 不覆盖运行期缓存 |

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
│  ├─ <entity>.param.go  List/Create/Update Input 参数      │
│  ├─ <provider>adapter/ 对外提供的适配器实现                 │
│  └─ stores/<domain>db/ 数据库实现 (被动适配器)             │
└──────────────────────────────────────────────────────────┘
```

**依赖方向**：API → Core ← Store/Adapter（外层依赖内层，内层通过接口反转依赖）

---

## 代码生成

CRUD 场景**必须**使用 `goddd gen` 生成代码。

### 步骤

1. 在 `tables/<domain>/` 下创建表定义文件
2. 结构体**必须包含** `ID`、`CreatedAt`、`UpdatedAt` 字段
3. 若使用随机字符 ID，使用 `uniqueid.Core` 类型
4. 同一领域多个结构体放在同一个 tables 文件中
5. 执行生成：`goddd gen -f tables/<domain>/<entity>.go`
6. 在 `internal/web/api/provider.go` 注册 Wire provider
7. 调用生成的 `Register<Domain>` 函数注册路由
8. 在领域目录下创建 `doc.go` 描述领域用途

### 表定义示例

```go
type Task struct {
    ID        uniqueid.Core `gorm:"primaryKey"`
    Name      string
    Status    int
    CreatedBy string
    Sort      int64  `gorm:"autoIncrement"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Wire 注册

```go
func NewTaskCore(db *gorm.DB) task.Core {
    store := taskdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return task.NewCore(store)
}
```

---

## EntityStorer 接口规范

每个实体生成的 Storer 接口定义在 `internal/core/<domain>/<entity>.go`：

```go
type EntityStorer interface {
    NewWithTx(orm.Tx) (EntityStorer, error)
    Create(context.Context, *Entity) error
    Update(context.Context, *Entity, func(*Entity) error) error
    Delete(context.Context, *Entity) error
    List(context.Context, *[]*Entity, *ListEntityInput) (int64, error)
    Count(context.Context, *ListEntityInput) (int64, error)
    GetByID(context.Context, int) (*Entity, error)
}
```

### 关键设计约束

| 规则 | 说明 |
|------|------|
| NewWithTx 跨域事务 | 传入 `orm.Tx` 返回事务副本，多个 Store 共享同一事务 |
| Update 原子性 | Store 内部用 `SELECT ... FOR UPDATE` + `Save`，保证读写原子 |
| Update 锁查询 | 使用 `Take(model)` 而非 `First(model)`，避免多余 ORDER BY |
| Delete 幂等 | `Clauses(clause.Returning{}).Delete(model)`，重复删除不报错 |
| 主键必填 panic | `model.ID == 0` 时 panic，强制调用方填充主键 |
| 无 ORM 泄露 | Core 层接口不含 `*gorm.DB`，仅通过 `orm.Tx` 抽象事务 |
| GetByID 命名 | 单条查询命名为 `GetByID`（非 QueryByID） |
| SortSafelist | 只需定义列名（如 `"id"`），`SortColumn()` 自动去除 `-` 前缀 |

### 跨域事务（NewWithTx 模式）

```go
// 在 Adapter 层协调跨域事务
tx, err := orm.Begin(db)
defer tx.Rollback()

txUserStore, _ := userStorer.NewWithTx(tx)
txOrderStore, _ := orderStorer.NewWithTx(tx)

txUserStore.Update(ctx, user, fn)
txOrderStore.Create(ctx, order)

tx.Commit()

// 便捷写法
orm.Transaction(db, func(tx orm.Tx) error {
    txUser, _ := userStorer.NewWithTx(tx)
    txUser.Update(ctx, model, fn)
    return nil
})
```

### Store 实现要点（db 层）

- `NewWithTx`：克隆 struct，内部 db 替换为 `orm.GormDB(tx)`
- 不显式写 `.Where("id = ?", model.ID)`，GORM 从非零主键自动推导 WHERE
- Update 事务内：`tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(model)` → `changeFn(model)` → `tx.Save(model)`
- Delete：`d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model)`
- Cache 层 `NewWithTx` 直接透传底层 db store 的事务副本（事务内绕过缓存）

---

## 参数定义规范

**核心原则**：归属字段（TenantID、CreatedBy）由 API 层填充，`json:"-"` / `form:"-"` 标记，编辑时不可修改。

```go
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
    Name      string `form:"name"`
    TenantID  string `form:"-"`
}

type CreateEntityInput struct {
    Name      string `json:"name" binding:"required,max=50"`
    TenantID  string `json:"-"`
    CreatedBy string `json:"-"`
}

type UpdateEntityInput struct {
    ID   int    `uri:"id"`
    Name string `json:"name"`
    // 不含归属字段；ID 来自路由参数
}

type GetEntityInput struct {
    ID int `uri:"id"`
}

type DeleteEntityInput struct {
    ID int `uri:"id"`
}
```

> 校验规范、分页过滤详见 `references/api-design-patterns.md`

---

## 领域间解耦

领域间**必须**通过适配器解耦，不能直接依赖其他领域的 Core。

| 规则 | 说明 |
|------|------|
| Port 定义在**提供方** | 接口和模型在 `<provider>adapter/` 子包 |
| Adapter 实现在**提供方** | 同上 |
| 消费方通过 **Option 注入** | `NewCore(store, opts...)` |
| 返回类型定义在**提供方子包** | 避免重复定义 |

### Option 注入模式

```go
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

> 完整代码模板（Port 定义、Adapter 实现、Wire 装配、port.go/model.go 分工）详见 `references/adapter-pattern.md`

---

## 领域内子包依赖

| 规则 | 说明 |
|------|------|
| 子包单向依赖根包 | 所有子包可 import 根包，根包不感知子包 |
| 子包间可直接依赖 | 无需通过接口隔离 |
| 禁止循环依赖 | 双向引用时一方用接口打破 |
| 基础设施仍需接口 | MQ、DB 驱动等外部依赖接口定义在根包 `port.go` |

### 判断是否需要接口

```
需要接口的场景：
├── 实现方在领域外（MQ 客户端、DB 驱动、跨领域适配器）
├── 子包间存在双向依赖（一方用接口打破循环）
└── 需要可测试性（mock 外部服务）

不需要接口的场景：
├── 子包 A 单向依赖子包 B（直接 import）
├── 子包依赖根包的类型/常量（直接引用）
└── 子包内部的辅助函数/工具类型
```

### 示例拓扑

```
domain/                 ← 领域根包（定义端口接口、模型、共享类型）
├── sub-a/              ← 单向依赖 domain
├── sub-b/              ← 单向依赖 domain
├── sub-infra/          ← 实现 domain.Publisher 等接口
└── stores/domaindb/    ← 实现 domain.Storer
```

子包之间若无交叉依赖，全部通过根包共享类型和接口定义。若 sub-a 需调用 sub-b 可直接 import；若反向也需要，则一方定义 narrow interface 打破循环。

---

## 排序功能实现

拖拽排序：接收有序 ID 数组，重新分配 sort 值，不影响未传入记录。

### 核心逻辑

1. 查询传入 ID 的记录，获取现有 `sort` 值
2. 将 `sort` 值升序排列
3. 按传入 ID 顺序重新分配排序值
4. 事务批量更新

### 要点

- 数据库字段 `sort` 使用 gorm tag `autoIncrement` 自增
- Store 层用事务批量更新，Core 层编排逻辑，API 层只做协议转换
- 校验所有 ID 存在，不存在则返回错误
- 路由：`g.PUT("/sort", web.WrapH(api.sortXxx))`

> 完整参数定义、Store/Core/API 三层代码模板详见 `references/sort.md`

---

## WithContext：Core 层获取 HTTP 信息

Core 不依赖 HTTP 框架，Adapter 需要 HTTP 元信息时，通过 `web.Context` 透传：

- **API 层**：`ctx := web.WithContext(c.Request)`
- **Core 层**：透传 `ctx context.Context`
- **Adapter 层**：`if wc, ok := ctx.(web.Context); ok { ... }`

### 设计要点

| 特性 | 说明 |
|------|------|
| 零破坏性 | 实现 `context.Context`，现有签名无需修改 |
| 渐进式采用 | 只改调用处（API）和使用处（Adapter），Core 层透传 |
| 优雅降级 | 断言失败返回原始值，非 HTTP 场景正常工作 |

> 完整背景分析、代码示例、适用/不适用场景详见 `references/with-context.md`

---

## Core 生命周期分离

**触发条件**：Core 需后台 goroutine，Wire 因值/指针类型冲突循环依赖。

| 结构体 | 类型 | 职责 |
|--------|------|------|
| `Core` | 值类型 | 纯业务逻辑（查询 DB、计算、编排） |
| `XxxHandler` | 指针，Core 内嵌 | goroutine 启动/停止、ctx 管理、优雅停机 |

### 方法分配

- **Core 方法**：查询 DB、纯业务计算
- **Core 委托方法**：一行转发给 Handler（`c.ss.TrackHeartbeat(...)`）
- **Handler 方法**：goroutine 内部私有逻辑

Wire：`func NewXxxCore(...) (xxx.Core, func()) { ... }`，返回值类型 Core + 清理函数。

> 完整结构定义、构造函数、反模式警告详见 `references/lifecycle-split.md`

---

## Store 缓存层规范

修改 `stores/<domain>cache/` 时，**首先**判断缓存类型：

| 类型 | 依赖 | 适用场景 |
|------|------|---------|
| 内存缓存 | `conc.Cacher` | 单副本、短 TTL、数据量小 |
| Redis 缓存 | `redis.Cmdable` | 多副本共享、长 TTL、高频读 |

若为 Redis 缓存：删除 `conc.Cacher`，换 `redis.Cmdable`（接口类型，兼容单机/集群），SETNX 防竞态。

```go
func NewXxxCore(db *gorm.DB, rdb redis.Cmdable) xxx.Core {
    dbStore := xxxdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    store := xxxcache.NewCache(dbStore, rdb)
    store.WarmUp(context.Background())
    return xxx.NewCore(store)
}
```

> 完整改造步骤、防竞态、穿透防护、Key 命名详见 `references/cache-layer.md`

---

## API 层规范

1. **只做协议转换**：参数绑定 → 填充归属字段 → 调用 Core → 返回响应
2. **归属字段 API 层填充**：`json:"-"` / `form:"-"`
3. **路由参数用 `uri` tag**：`struct{ ID string \`uri:"id"\` }`

```go
func registerTask(r gin.IRouter, api TaskAPI, handler ...gin.HandlerFunc) {
    g := r.Group("/tasks", handler...)
    g.GET("", web.WrapH(api.listTasks))
    g.POST("", web.WrapH(api.createTask))
    g.GET("/:id", web.WrapH(api.getTask))
    g.PUT("/:id", web.WrapH(api.updateTask))
    g.DELETE("/:id", web.WrapH(api.deleteTask))
    g.PUT("/sort", web.WrapH(api.sortTasks))
}
```

### WrapH 入参规则

- POST/PUT/DELETE → 绑定 Request Body（`json` tag）
- GET → 绑定 URL Query（`form` tag）
- 文件上传 → multipart form 统一绑定，in 结构体内嵌 `File *multipart.FileHeader \`form:"file"\``，禁止 `c.Request.FormFile` 手取（模板见 `references/web-toolkit.md`）
- 入参第二个参数必须是指针，`*struct{}` 表示无参数
- 路由参数通过 `uri` tag 自动绑定：`struct{ ID string \`uri:"id"\` }`

### 错误处理

Core 层返回 `reason.Error`，`web.WrapH` 自动映射 HTTP 状态码：

```go
return nil, reason.ErrBadRequest.WithMsg("参数不合法")     // → 400
return nil, reason.ErrDB.Withf("查询失败: %s", err)       // → 400
return nil, reason.ErrUnauthorized.WithMsg("未登录")   // → 401
```

- `SetMsg()` — 给用户的友好提示
- `Withf()` — 给开发者的 details（`SetRelease()` 后不输出）

> 完整 API 设计规范（资源命名、标准方法、自定义方法、状态码、分页、校验、限流）详见 `references/api-design-patterns.md`

### API 文档同步（联动 goddd-api-doc 技能）

凡涉及以下变动，**必须**使用 `goddd-api-doc` 技能同步更新 `docs/api/*.go.yaml` 接口文档（不等用户要求）：

- handler 函数签名变更（入参/出参类型）
- 路由路径或 HTTP 方法变更
- 请求/响应结构体字段增删改
- model 字段与 JSON 映射变更（重命名、类型变更）
- 新增接口后尚无对应 `.go.yaml` 文件

## 版本变化

旧版本目录名为 store，如果遇到应该迁移为 stores