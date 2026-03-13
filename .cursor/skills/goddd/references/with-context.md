# WithContext 架构设计

## 背景

六边形架构中 Core 层不应依赖 HTTP 框架，但 Adapter 有时需要 HTTP 请求元信息（如拼接完整图片 URL）。

常见方案各有缺陷：

| 方案 | 问题 |
|------|------|
| API 层逐个字段后处理 | 每个接口重复写，散落各处 |
| 函数签名传入 `*http.Request` | 侵入性强，所有调用链都要改签名 |
| 函数签名传入 `baseURL string` | 每多一个需求就多加一个参数 |
| 直接传入 `gin.Context` | Core 层对 HTTP 框架产生耦合 |

## 设计方案

`web.Context` 扩展了 `context.Context`，携带 `*http.Request`：

```go
type Context interface {
    context.Context
    Request() *http.Request
    GetBaseURL() string
    GetScheme() string
    GetHost() string
}

func WithContext(r *http.Request) Context {
    return &httpRequestContext{Context: r.Context(), req: r}
}
```

## 使用方式

### API 层 — 构造 web.Context

```go
func (a XxxAPI) findItems(c *gin.Context, in *FindInput) (*web.PageOutput[*Item], error) {
    ctx := web.WithContext(c.Request)  // 替代 c.Request.Context()
    return a.core.FindItems(ctx, in)
}
```

### Core 层 — 透传 ctx

Core 层完全不感知 HTTP，只是透传 `ctx context.Context`：

```go
func (c Core) FindItems(ctx context.Context, in *FindInput) (*web.PageOutput[*Item], error) {
    items, total, err := c.store.Find(ctx, in)
    // Adapter 内部会通过类型断言获取 HTTP 信息
    c.enrichItems(ctx, items)
    return &web.PageOutput[*Item]{Items: items, Total: total}, err
}
```

### Adapter 层 — 类型断言获取 HTTP 信息

```go
func (p *impl) resolveCover(ctx context.Context, cover string) string {
    if cover == "" || strings.HasPrefix(cover, "http") {
        return cover
    }
    if p.coverURLFunc == nil {
        return cover
    }
    if wc, ok := ctx.(web.Context); ok {
        return p.coverURLFunc(wc.Request(), cover)
    }
    return cover  // 非 HTTP 场景降级返回相对路径
}
```

## 数据流

```
API 层                      Adapter (BriefProvider)
  │                              │
  │ ctx = web.WithContext(req)   │
  │──────── Core 透传 ctx ──────>│
  │                              │ ctx.(web.Context) → req
  │                              │ coverURLFunc(req, relativePath) → fullURL
  │<──────────── Brief{Cover: fullURL}
```

## 设计原则

| 特性 | 说明 |
|------|------|
| 零破坏性 | 实现 `context.Context`，现有签名无需修改 |
| 渐进式采用 | 只改调用处（API）和使用处（Adapter） |
| 优雅降级 | 断言失败返回原始值，定时任务/测试/CLI 正常工作 |
| 可扩展 | 可定义子接口扩展，如 `TenantContext` |

## 适用场景

| 场景 | 说明 |
|------|------|
| 动态 URL 拼接 | 需要当前请求的 scheme + host |
| 请求级元信息透传 | IP 地址、User-Agent |
| 跨领域 Adapter 数据转换 | 需要 HTTP 上下文辅助 |

## 不适用场景

| 场景 | 建议 |
|------|------|
| 简单已有参数 | 直接传参更清晰 |
| 与 HTTP 无关的逻辑 | 标准 `context.Context` |
| 需修改请求状态 | 应在 API 层处理 |
