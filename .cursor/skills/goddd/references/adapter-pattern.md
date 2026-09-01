# 适配器模式与跨域解耦

## 设计原则

领域间必须通过适配器解耦，禁止直接依赖其他领域的具体实现：

- **Port（接口）** 定义在提供方子包（如 `user/useradapter/`）或本领域的 `port.go`
- **Adapter（实现）** 放在提供方子包（如 `user/useradapter/`）
- **消费方** 通过 Option 注入适配器接口
- **返回类型** 定义在提供方子包，避免消费方重复定义模型

---

## 目录结构

```
internal/core/user/              # 提供方
├── core.go
├── user.model.go
└── useradapter/                 # 提供方的适配器子包
    └── useradapter.go           # 接口 + 模型 + 实现

internal/core/message/           # 消费方
├── core.go                      # 通过 Option 注入 BriefProvider
├── port.go                      # 本领域自己的 Port（如 ContentProvider）
└── ...
```

---

## 提供方：定义接口和模型

```go
// user/useradapter/useradapter.go

package useradapter

import (
    "context"
    "net/http"
    "strings"
    "github.com/ixugo/goddd/pkg/web"
)

type Brief struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Cover string `json:"cover"`
}

type BriefProvider interface {
    GetUserBrief(ctx context.Context, userID string) (*Brief, error)
    GetUserBriefs(ctx context.Context, userIDs []string) (map[string]*Brief, error)
}

type CoverURLResolver func(r *http.Request, storagePath string) string
```

---

## 提供方：实现适配器

```go
// user/useradapter/useradapter.go

type briefProviderImpl struct {
    userCore     user.Core
    coverURLFunc CoverURLResolver
}

func NewBriefProvider(userCore user.Core, coverURLFunc CoverURLResolver) BriefProvider {
    return &briefProviderImpl{
        userCore:     userCore,
        coverURLFunc: coverURLFunc,
    }
}

func (p *briefProviderImpl) GetUserBrief(ctx context.Context, userID string) (*Brief, error) {
    if userID == "" {
        return nil, nil
    }
    u, err := p.userCore.GetUser(ctx, userID)
    if err != nil {
        return nil, nil // 用户不存在时返回 nil，不阻断主流程
    }
    return &Brief{
        ID:    u.ID,
        Name:  u.Name,
        Cover: p.resolveCover(ctx, u.Cover),
    }, nil
}

func (p *briefProviderImpl) resolveCover(ctx context.Context, cover string) string {
    if cover == "" || strings.HasPrefix(cover, "http") {
        return cover
    }
    if p.coverURLFunc == nil {
        return cover
    }
    if wc, ok := ctx.(web.Context); ok {
        return p.coverURLFunc(wc.Request(), cover)
    }
    return cover
}
```

---

## 消费方：Option 注入

```go
// message/core.go

type Core struct {
    store            Storer
    contentProviders map[string]ContentProvider
    userProvider     useradapter.BriefProvider
}

type Option func(*Core)

func WithContentProvider(msgType string, provider ContentProvider) Option {
    return func(c *Core) {
        if c.contentProviders == nil {
            c.contentProviders = make(map[string]ContentProvider)
        }
        c.contentProviders[msgType] = provider
    }
}

func WithUserProvider(provider useradapter.BriefProvider) Option {
    return func(c *Core) {
        c.userProvider = provider
    }
}

func NewCore(store Storer, opts ...Option) Core {
    c := Core{
        store:            store,
        contentProviders: make(map[string]ContentProvider),
    }
    for _, opt := range opts {
        opt(&c)
    }
    return c
}
```

---

## 跨域事务协调（WithTx 模式）

| 模式 | 耦合 | 适用场景 | 事务发起者 |
|------|------|---------|-----------|
| **模式 A：Adapter 协调** | 低 | 两个对等域，无主从关系 | Adapter 持有多个 Storer 并发起事务 |
| **模式 B：Core 内部编排** | 中 | 有明确主域（A 强依赖 B） | 主域 Core 自行 Begin 并将 tx 传递给从域 Storer |

### 模式 A — Adapter 协调（对等域）

```go
// order/orderadapter/orderadapter.go

type OrderTxCoordinator struct {
    orderStore order.Storer
    userStore  user.Storer
}

func NewOrderTxCoordinator(orderStore order.Storer, userStore user.Storer) *OrderTxCoordinator {
    return &OrderTxCoordinator{orderStore: orderStore, userStore: userStore}
}

func (c *OrderTxCoordinator) CreateOrderAndDeduct(ctx context.Context, in Input) error {
    tx, err := c.orderStore.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    txOrder := c.orderStore.Order().WithTx(tx)
    txUser := c.userStore.User().WithTx(tx)

    if err := txOrder.Create(ctx, in.Order); err != nil {
        return err
    }
    if err := txUser.DeductBalance(ctx, in.UserID, in.Amount); err != nil {
        return err
    }
    return tx.Commit()
}
```

### 模式 B — Core 内部编排（主从域）

主域 Core 通过 Option 注入从域的 EntityStorer，内部发起事务：

```go
type Core struct {
    store      Storer
    userStorer user.UserStorer // Option 注入
}

func WithUserStorer(s user.UserStorer) Option {
    return func(c *Core) { c.userStorer = s }
}

func (c Core) CreateOrderAndDeduct(ctx context.Context, in Input) error {
    tx, err := c.store.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    txOrder := c.store.Order().WithTx(tx)
    txUser := c.userStorer.WithTx(tx)

    if err := txOrder.Create(ctx, in.Order); err != nil {
        return err
    }
    if err := txUser.DeductBalance(ctx, in.UserID, in.Amount); err != nil {
        return err
    }
    return tx.Commit()
}
```

**选择依据**：
- 两域无从属、仅特定操作需要原子性 → **模式 A**
- 主域强依赖从域且频繁调用 → **模式 B**
- 两者底层均走 `Begin()` + `WithTx(tx)`
