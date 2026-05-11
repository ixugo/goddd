# WithContext 架构设计

## 背景

在六边形架构中，Core 层不应依赖 HTTP 框架（如 `gin.Context`），但某些场景需要 HTTP 请求元信息来构造动态数据（如完整的图片 URL、当前请求的基地址等）。

常见的解决方式各有缺陷：

| 方案 | 问题 |
|------|------|
| API 层逐个字段后处理 | 每个接口都要重复写，散落在各处 |
| 函数签名传入 `*http.Request` | 侵入性强，所有调用链都要改签名 |
| 函数签名传入 `baseURL string` | 每多一个需求就多加一个参数 |
| 直接传入 `gin.Context` | Core 层对 HTTP 框架产生耦合 |

## 设计方案：`web.Context`

在 `goweb` 库中定义一个扩展了 `context.Context` 的接口：

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

下游通过类型断言 `ctx.(web.Context)` 按需获取 HTTP 元信息：

```go
if wc, ok := ctx.(web.Context); ok {
    baseURL := wc.GetBaseURL()
    // 使用 baseURL 拼接完整 URL
}
```

## 设计原则

### 1. 零破坏性

`web.Context` 实现了 `context.Context` 接口，所有现有函数签名 `func(ctx context.Context, ...)` 无需修改即可接收 `web.Context`。

### 2. 渐进式采用

改造只涉及两处：
- **调用处**（API 层）：`ctx := web.WithContext(c.Request)` 替代 `c.Request.Context()`
- **使用处**（Adapter 层）：`if wc, ok := ctx.(web.Context); ok { ... }`

中间层（Core 层）完全不需要感知，只是透传 `ctx`。

### 3. 优雅降级

类型断言失败时返回原始值，非 HTTP 场景（定时任务、单元测试、CLI 工具）正常工作：

```go
func (p *impl) resolveCover(ctx context.Context, cover string) string {
    if wc, ok := ctx.(web.Context); ok {
        return p.coverURLFunc(wc.Request(), cover)
    }
    return cover // 降级：返回相对路径
}
```

### 4. 可扩展性

`web.Context` 是接口，未来新增方法不影响已有代码。也可以定义新的子接口按需扩展：

```go
type TenantContext interface {
    Context
    GetTenantID() string
}
```

## 与 BriefProvider 结合使用

`web.Context` 与 `useradapter.BriefProvider` 配合，解决了「用户头像 URL 需要 HTTP 请求才能拼接完整地址」的问题：

```
API 层                      BriefProvider (Adapter)
  │                              │
  │ ctx = web.WithContext(req)   │
  │──────────────────────────────>│
  │                              │ ctx.(web.Context) → req
  │                              │ coverURLFunc(req, relativePath) → fullURL
  │<─────────────── Brief{Cover: fullURL}
```

- API 层只需传入 `web.WithContext(c.Request)` 作为 `ctx`
- Core 层透传 `ctx`，完全不感知 HTTP
- BriefProvider 通过类型断言获取 `*http.Request`，调用注入的 `CoverURLResolver` 拼接完整 URL
- 非 HTTP 场景降级返回相对路径

## 适用场景

| 场景 | 说明 |
|------|------|
| 动态 URL 拼接 | 需要当前请求的 scheme + host 构造完整地址 |
| 请求级元信息透传 | IP 地址、User-Agent 等需要传入 Core 层的场景 |
| 跨领域数据组装 | Adapter 需要 HTTP 上下文辅助数据转换 |

## 不适用场景

| 场景 | 建议 |
|------|------|
| 已有明确参数的简单场景 | 直接传参数更清晰 |
| 与 HTTP 完全无关的逻辑 | 使用标准 `context.Context` |
| 需要修改请求状态 | 应在 API 层处理，不应下沉 |
