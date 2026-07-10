# API 设计模式（goddd 项目适配版）

本项目 API 遵循六边形架构（goddd），设计规范以项目实际用法为准。

## 资源命名

- 用复数名词：`/users`、`/orders`、`/products`
- 嵌套表示关系：`/users/{id}/orders`，最大嵌套 2 层
- 用 kebab-case：`/user-profiles`，不用 `/userProfiles`
- URL 中不放动词：用 `POST /users/{id}/activation`，不用 `GET /users/{id}/activate`

## HTTP 方法与状态码

| 方法 | 用途 | 幂等 | WrapH 绑定方式 | 成功码 |
|------|------|------|---------------|--------|
| GET | 读取资源 | 是 | `form` tag → Query | 200 |
| POST | 创建资源 | 否 | `json` tag → Body | 201 |
| PUT | 完整替换/排序 | 是 | `json` tag → Body | 200 |
| PATCH | 部分更新 | 否 | `json` tag → Body | 200 |
| DELETE | 删除资源 | 是 | `json` tag → Body（可选） | 204 |

路由参数通过 `uri` tag 自动绑定：`struct{ ID string \`uri:"id"\` }`。

## 错误处理

项目使用 `reason.Error` 体系，`web.WrapH` 自动映射 HTTP 状态码：

```go
reason.ErrBadRequest.SetMsg("参数不合法")     // → 400
reason.ErrUnauthorized.SetMsg("未登录")       // → 401
reason.ErrForbidden.SetMsg("无权操作")        // → 403
reason.ErrNotFound.SetMsg("资源不存在")       // → 404
reason.ErrConflict.SetMsg("状态冲突")         // → 409
reason.ErrDB.Withf("查询失败: %s", err)       // → 500
```

- `SetMsg()` — 用户可见的友好提示
- `Withf()` — 开发调试用的 details（`SetRelease()` 后不输出）

## 分页

### 偏移分页（项目默认）

使用 `web.PagerFilter`：

```go
type ListEntityInput struct {
    web.PagerFilter
    Name string `form:"name"`
}
```

请求：`GET /entities?page=1&size=20&sort=-created_at`

响应用 `web.PageOutput[T]`：`{"items": [...], "total": 42}`

### 不分页查询

```go
pf := web.NewPagerFilterMaxSize()
```

### 排序安全

`PagerFilter.SortSafelist` 白名单控制可排序字段，防注入。排序用 `-` 前缀降序、`+` 或无前缀升序。

## 日期范围过滤

使用 `web.DateFilter`：

```go
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
}
```

请求：`GET /entities?start_ms=1720000000000&end_ms=1720100000000`

`DateFilter` 字段为毫秒时间戳（`orm.Time`），通过 `StartAt()` / `EndAt()` 获取 `time.Time`。

## 归属字段

TenantID、CreatedBy 等归属字段由 API 层从 JWT 上下文填充：

```go
func (a EntityAPI) createEntity(c *gin.Context, in *entity.CreateEntityInput) (*entity.Entity, error) {
    in.TenantID = web.GetUID(c)
    in.CreatedBy = web.GetUsername(c)
    return a.core.CreateEntity(c.Request.Context(), *in)
}
```

参数结构体中标记 `json:"-"` / `form:"-"`，禁止调用方传入。

## 版本控制

URL 路径版本控制：
```
/api/v1/users
/api/v2/users
```

- v1 发布后只增字段，不删不改已有字段
- 新的必填字段 = 新版本
- 同时最多维护 2 个活跃版本
- 旧版本标记 `deprecated: true`，附弃用说明

## 限流

项目中间件提供三级限流：

| 中间件 | 粒度 | 用法 |
|--------|------|------|
| `web.RateLimiter(r, b)` | 全局 | 入口处挂载 |
| `web.IPRateLimiterForGin(r, b)` | 按 IP | 公开接口 |
| `web.IDRateLimiter(r, b, ttl)` | 按用户 ID | 认证后接口 |

被限流返回 `429 Too Many Requests`，带 `Retry-After` 头。

## 路由注册模板

```go
func registerEntity(r gin.IRouter, api EntityAPI, handler ...gin.HandlerFunc) {
    g := r.Group("/entities", handler...)
    g.GET("", web.WrapH(api.listEntities))      // 列表
    g.POST("", web.WrapH(api.createEntity))      // 创建
    g.GET("/:id", web.WrapH(api.getEntity))      // 详情
    g.PUT("/:id", web.WrapH(api.updateEntity))   // 更新
    g.DELETE("/:id", web.WrapH(api.deleteEntity)) // 删除
    g.PUT("/sort", web.WrapH(api.sortEntities))   // 排序
}
```

## 校验

使用 `web.Validator` 在业务函数内手动校验：

```go
func (c Core) CreateEntity(ctx context.Context, in CreateEntityInput) (*Entity, error) {
    v := web.NewValidator()
    v.Check(in.Name != "", "name", "名称不能为空")
    v.Check(len(in.Name) <= 50, "name", "名称不超过 50 字")
    if err := v.Valid(); err != nil {
        return nil, err
    }
    // ...
}
```

`WrapH` 仅负责参数绑定，不自动调用校验。校验逻辑放在 Core 层业务方法中。
