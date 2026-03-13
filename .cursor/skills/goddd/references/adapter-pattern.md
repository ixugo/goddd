# 适配器模式与 Option 注入

## 设计原则

领域间必须通过适配器解耦：

- **Port（接口）** 定义在提供方子包（如 `user/useradapter/`）
- **Adapter（实现）** 也放在提供方子包
- **消费方** 通过 Option 注入适配器实例
- **返回类型** 定义在提供方子包，避免重复

当多个消费方需要同一提供方的同一能力时，集中在提供方可避免重复代码。

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

## 提供方：定义接口和模型

```go
// user/useradapter/useradapter.go

package useradapter

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
        return nil, nil  // 用户不存在时返回 nil，不返回 error
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

## API 层：Wire 注入

```go
// internal/web/api/message.go

func NewMessageCore(
    db *gorm.DB,
    commentProvider message.ContentProvider,
    taskProvider message.ContentProvider,
    mediaProvider message.ContentProvider,
    briefProvider useradapter.BriefProvider,
) message.Core {
    store := messagedb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return message.NewCore(store,
        message.WithContentProvider(message.TypeComment, commentProvider),
        message.WithContentProvider(message.TypeTask, taskProvider),
        message.WithContentProvider(message.TypeMedia, mediaProvider),
        message.WithUserProvider(briefProvider),
    )
}
```

## Port 定义位置（port.go）

本领域自己的被动适配器接口定义在 `port.go`，与 `model.go` 分离：

```go
// message/port.go

type ContentProvider interface {
    GetContent(ctx context.Context, targetID string) (any, error)
}
```

## model.go 只放类型定义

```go
// message/model.go

const (
    TypeComment = "comment"
    TypeTask    = "task"
    TypeMedia   = "media"
)

type MessageBrief struct {
    Content  any                 `json:"content"`
    Creator  *useradapter.Brief  `json:"creator"`
}
```
