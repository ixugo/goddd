# 领域层架构与 Storer 规范

本文档详述 goddd 架构中 Core 层、Store 层（db/cache）的设计规范、Storer/EntityStorer 接口定义、事务机制以及零分配访问器原理。

---

## 领域层架构全貌

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
│  ├─ port.go            被动适配器接口                      │
│  ├─ doc.go             领域说明                           │
│  ├─ model.go           非 GORM 类型定义                   │
│  ├─ <entity>.go        业务方法 + EntityStorer 实体接口    │
│  ├─ <entity>.model.go  领域模型 (GORM 映射)               │
│  ├─ <entity>.param.go  List/Create/Update Input 参数      │
│  ├─ <provider>adapter/ 对外提供的适配器实现                 │
│  ├─ stores/<domain>db/ 数据库实现 (被动适配器)             │
│  └─ stores/<domain>cache/ 缓存实现 (被动适配器)           │
└──────────────────────────────────────────────────────────┘
```

**依赖方向**：API → Core ← Store/Adapter（外层依赖内层，内层通过接口反转依赖，Core 不依赖任何底层 ORM 或具体数据库驱动）。

---

## Storer 聚合接口

每个领域生成的聚合接口定义在 `internal/core/<domain>/core.go`：

```go
type Storer interface {
    Begin() (orm.Tx, error)
    User() UserStorer
    Order() OrderStorer
}
```

### 设计要点

1. **事务入口抽象**：`Begin()` 由 DB 层实现（`orm.Begin(d.db)`），Cache 层直接透传底层 DB 的 Begin。Core 层通过 `c.store.Begin()` 发起事务，不接触 `*gorm.DB`。
2. **访问器零分配设计**：
   - **DB 层（类型转换零分配）**：DB 结构体保持单字段 `type DB struct { db *gorm.DB }`。访问器实现为 `func (d DB) User() UserStorer { return User(d) }`，其中 `type User DB`。单指针字段结构体在 Go 中符合 directIface，装箱入接口时不发生堆分配。**严禁向 DB 结构体添加其他字段**，否则会导致隐式退化为堆分配。
   - **Cache 层（构造时预建）**：Cache 结构体包含多个字段（store、rdb、sf 等），直接类型转换必然逃逸分配。因此子 storer 在 `NewCache` 构造时预先实例化并保存在 Cache 字段中，访问器直接返回已建好的字段。

---

## EntityStorer 实体接口规范

每个实体生成的 Storer 接口定义在 `internal/core/<domain>/<entity>.go`：

```go
type EntityStorer interface {
    WithTx(orm.Tx) EntityStorer
    Create(context.Context, *Entity) error
    Update(context.Context, *Entity, func(*Entity) error) error
    Delete(context.Context, *Entity) error
    List(context.Context, *ListEntityInput) ([]*Entity, int64, error)
    Count(context.Context, *ListEntityInput) (int64, error)
    GetByID(context.Context, int) (*Entity, error)
}
```

### 关键设计约束

| 规则 | 说明 |
|------|------|
| `Storer.Begin()` | 聚合接口提供事务入口，Core 层即可发起事务 |
| `WithTx` 跨域事务 | 传入 `orm.Tx` 返回事务副本，多个 Store 共享同一底层事务 |
| `WithTx` 返回指针/副本 | 每事务一次、非热路径；Cache 层返回 `&Entity{...}`，DB 层返回 `Entity{db: orm.GormDB(tx)}` |
| `Update` 原子性 | Store 内部用 `SELECT ... FOR UPDATE` + `changeFn` + `Save`，保证读写原子 |
| `Update` 锁查询 | 使用 `Take(model)` 而非 `First(model)`，避免多余的 ORDER BY 排序开销 |
| `Delete` 幂等 | `Clauses(clause.Returning{}).Delete(model)`，重复删除不报错，返回被删实体 |
| 主键必填 panic | 当 `model.ID == 0`（或空字符串）时直接 panic，强制调用方填充主键，防止全表误操作 |
| 无 ORM 泄露 | Core 层接口不含 `*gorm.DB`，仅通过 `orm.Tx` 抽象事务 |
| `GetByID` 命名 | 单条主键查询统一命名为 `GetByID`（非 QueryByID / FindByID） |
| `SortSafelist` | 只需定义安全列名（如 `"id"`、`"created_at"`），`SortColumn()` 自动去除 `-` 前缀并校验 |

---

## DB Store 核心实现要点

```go
// 1. WithTx 实现
func (d User) WithTx(tx orm.Tx) user.UserStorer {
    return User{db: orm.GormDB(tx)}
}

// 2. Update 事务内锁行更新
func (d User) Update(ctx context.Context, model *user.User, changeFn func(*user.User) error) error {
    if model.ID == 0 {
        panic("userdb.Update: model.ID is zero, primary key is required")
    }
    return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(model).Error; err != nil {
            return err
        }
        if err := changeFn(model); err != nil {
            return err
        }
        return tx.Save(model).Error
    })
}

// 3. Delete 幂等删除
func (d User) Delete(ctx context.Context, model *user.User) error {
    if model.ID == 0 {
        panic("userdb.Delete: model.ID is zero, primary key is required")
    }
    return d.db.WithContext(ctx).Clauses(clause.Returning{}).Delete(model).Error
}

// 4. List 分页查询与排序白名单
func (d User) List(ctx context.Context, in *user.ListUserInput) ([]*user.User, int64, error) {
    in.SortSafelist = []string{"id", "created_at"}
    db := applyUserFilter(d.db, in)
    orderBy := "created_at DESC"
    if col := in.MustSortColumn(); col != "" {
        orderBy = col
    }
    db = db.Order(orderBy)

    var total int64
    items := make([]*user.User, 0)
    db = db.Model(new(user.User)).WithContext(ctx)
    if err := db.Count(&total).Error; err != nil || total <= 0 {
        return items, total, err
    }
    err := db.Limit(in.Limit()).Offset(in.Offset()).Find(&items).Error
    return items, total, err
}
```

---

## 参数定义规范（Input 结构体）

**核心原则**：归属字段（TenantID、CreatedBy）由 API 层从上下文或 Token 提取并填充，使用 `json:"-"` / `form:"-"` 标记，禁止由外部请求直接覆盖。

```go
// 列表查询入参
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
    Name     string `form:"name"`
    TenantID string `form:"-"` // API 层从 JWT 注入
}

// 创建记录入参
type CreateEntityInput struct {
    Name      string `json:"name" binding:"required,max=50"`
    TenantID  string `json:"-"`
    CreatedBy string `json:"-"`
}

// 更新记录入参
type UpdateEntityInput struct {
    ID   int    `uri:"id"`
    Name string `json:"name" binding:"max=50"`
    // 不包含归属字段；ID 来自路由参数
}

// 主键获取入参
type GetEntityInput struct {
    ID int `uri:"id"`
}

// 删除入参
type DeleteEntityInput struct {
    ID int `uri:"id"`
}
```

---

## 版本变化与迁移清单

遇到旧代码时按以下清单迁移：

| 旧写法 | 新写法 | 说明 |
|--------|--------|------|
| `store/` 目录 | `stores/` | 复数形式，含 `xxxdb/` + `xxxcache/` |
| `gin.CustomRecover` | `web.Recover` | 统一 recover 中间件 |
| `.SetMsg("xxx")` | `.WithMsg("xxx")` | `reason.CustomError` 友好提示不可变方法 |
| `.SetHTTPStatus(code)` | `.WithHTTPStatus(code)` | 覆盖状态码不可变方法 |
| `ErrRateLimit` / `ErrUnauthorizedToken` | `ErrTooManyRequests` / `ErrUnauthorized` | 废弃旧错误变量 |
| `Session(*gorm.DB)` / `UpdateWithSession` | `WithTx(orm.Tx)` | 事务模式重构，Core 层不再依赖 gorm |
| `.Where("id=?", model.ID).Delete(model)` | `.Delete(model)` | GORM 自动推导非零主键 WHERE |
| `stores/xxxdb/entity.go` | `stores/xxxdb/entity.db.go` | 生成文件命名含层级后缀 `.db.go` |
| `List(ctx, *[]*T, in) (int64, error)` | `List(ctx, in) ([]*T, int64, error)` | 出参改返回值 |
| `orm.ListWithContext` / `orm.List` / `orm.Find` / `orm.Pager` | Store 内直接 `Count` + `Limit(in.Limit()).Offset(in.Offset()).Find` | `pkg/orm/old.go` 全部函数已弃用，禁止新增引用 |
| Cache 层访问器内 `return &Xxx{...}` | 子 storer 于 `NewCache` 构造时预建为字段，访问器直接返回字段 | 规避每次调用堆分配 |
