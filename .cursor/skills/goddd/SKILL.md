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

> **核心法则**：遇到具体业务实现或不确定细节时，**必须根据索引先读取 `references/` 下的对应专题文档**，严禁凭空推测。

---

## 架构概览与分层

```
┌──────────────────────────────────────────────────────────┐
│                   API 层 (主动适配器)                      │
│  internal/web/api/                                       │
│  职责: HTTP 协议转换 → 填充归属字段 → 调用 Core → 返回响应   │
└──────────────────────┬───────────────────────────────────┘
                       │ 依赖
                       ▼
┌──────────────────────────────────────────────────────────┐
│               Core 层 (领域层/业务核心)                    │
│  internal/core/<domain>/                                 │
│                                                          │
│  ├─ core.go            Core 结构体 + Storer 聚合接口      │
│  ├─ port.go            被动适配器接口 (外部系统/MQ等)       │
│  ├─ doc.go             领域边界与用途说明                 │
│  ├─ model.go           非 GORM 领域模型/常量定义           │
│  ├─ <entity>.go        业务方法 + EntityStorer 实体接口    │
│  ├─ <entity>.model.go  实体持久化模型 (GORM 映射)          │
│  ├─ <entity>.param.go  List/Create/Update Input 参数      │
│  ├─ <provider>adapter/ 对外提供的适配器实现                 │
│  ├─ stores/<domain>db/ 数据库实现 (实现 Storer 接口)       │
│  └─ stores/<domain>cache/ 缓存实现 (实现 Storer 接口)     │
└──────────────────────────────────────────────────────────┘
```

**依赖方向**：API → Core ← Store/Adapter（外层依赖内层，内层通过接口反转依赖，Core 绝不感知具体 ORM 或 DB 驱动）。

---

## References 专题文档索引

实现对应功能时**必须首先读取**对应的专题参考文档：

| 文档路径 | 必读时机 | 核心内容 |
|---------|---------|---------|
| `references/code-generation.md` | 新增 CRUD、定义数据库表模型、Wire 注册 | `tables/` 目录规范、主键与时间戳约束、`goddd gen` 命令、Wire 注入、路由注册 |
| `references/domain-layer-architecture.md` | 实现/修改领域 Core、Store 接口与事务 | Storer 聚合接口、EntityStorer 规范、事务机制（`Begin/WithTx`）、访问器零分配原理、原子更新、幂等删除、Input 参数定义 |
| `references/api-design-patterns.md` | 设计新接口、审查 API 规范、错误映射 | 资源命名、标准方法（List/Get/Create/Update/Delete）、自定义方法、错误体系、分页过滤、校验、限流 |
| `references/web-toolkit.md` | 使用 `pkg/web` 中的工具函数与中间件 | WrapH 绑定规则、PagerFilter/DateFilter、JWT 鉴权、日志/限流/SSE 中间件、Validator |
| `references/adapter-pattern.md` | 新增跨领域依赖、跨域事务协调 | Port/Adapter 定义位置、Option 注入模式、Wire 装配、跨域事务协调（模式 A 与模式 B） |
| `references/cache-layer.md` | 修改或扩展 `stores/<domain>cache/` | 内存 vs Redis 缓存选型、SETNX/SETEX 防竞态、singleflight 防击穿、WarmUp 预热、事务副本语义 |
| `references/package-dependency.md` | 领域内拆分子包、处理子包间依赖 | 子包单向依赖根包原则、同级子包直接引用、Narrow Interface 打破双向循环依赖 |
| `references/event-notification.md` | 跨领域异步通知、解耦副作用 | `pkg/event` 泛型总线、观察者注册、Wire 注入、River 持久化异步队列集成 |
| `references/sort.md` | 实现列表拖拽重排序 | 有序 ID 数组 → 收集现有 sort 升序 → 重分配赋值 → 事务批量更新 |
| `references/with-context.md` | Core 或 Adapter 需要 HTTP 上下文信息 | `web.WithContext` 包装 → Core 透传标准 `ctx` → Adapter 类型断言解包 |
| `references/lifecycle-split.md` | Core 需后台 goroutine 且 Wire 循环依赖 | Core 值类型 + SessionHandler 指针内嵌、生命周期与业务分离、优雅停机 |
| `references/refactor-migration.md` | 重构/迁移旧代码到 goddd 架构 | SQL 条件、默认排序、错误控制流、空列表 `[]`、缓存失效等价性检查清单 |

---

## 核心速查与开发铁律

### 1. 代码生成与表定义（→ `code-generation.md`）
- 表模型放在 `tables/<domain>/<entity>.go`，必须含 `ID`、`CreatedAt`、`UpdatedAt`。
- 执行生成：`goddd gen -f tables/<domain>/<entity>.go`。
- 生成后于 `internal/web/api/provider.go` 注册 Wire，并在 `api.go` 注册路由。

### 2. 存储与事务铁律（→ `domain-layer-architecture.md`）
- **事务抽象**：Core 层通过 `c.store.Begin()` 获取 `orm.Tx`，再调用 `WithTx(tx)` 传递给各 Storer，不接触 `*gorm.DB`。
- **访问器零分配**：DB 结构体恒守单字段（`db *gorm.DB`），`return Xxx(d)` 零分配；Cache 结构体多字段，子 storer 必须在 `NewCache` 构造时预建为字段直接返回。
- **原子更新与幂等删除**：Update 必须在事务内 `SELECT ... FOR UPDATE` 加锁后通过 `changeFn` 修改；Delete 必须使用 `clause.Returning{}` 实现幂等。

### 3. API 错误处理规范（→ `api-design-patterns.md`）
- 错误使用 `reason.CustomError` 体系，统一使用不可变方法：
  - `WithMsg("提示信息")`：覆盖面向用户的提示。
  - `Withf("格式化信息", ...)`：追加开发者排查 details（生产环境不暴露）。
  - `WithHTTPStatus(code)`：覆盖默认状态码。
  - `WithCause(err)`：包裹底层错误链。
- 状态码映射：客户端业务错误默认 400，未登录 401 (`ErrUnauthorized`)，无权限 403 (`ErrPermissionDenied`)，限流 429 (`ErrTooManyRequests`)，服务端/数据库异常 500 (`ErrDB` / `ErrServer`)。

### 4. 缓存层防竞态（→ `cache-layer.md`）
- 读穿透回填用 `singleflight.Do` + `SetNX`；Create 不写缓存；Update 写完 DB 用 `Set` 覆盖；WarmUp 用 `SetNX`。
- `WithTx` 事务副本内：写操作仅失效缓存（Del/墓碑），读操作直连 DB。

### 5. API 文档同步联动
- 接口签名、路由、请求/响应结构体变更时，**必须联动 `goddd-api-doc` 技能** 同步更新 `docs/api/*.go.yaml`。
