# API 设计规范

goddd 的 API 设计规范，按此执行即可。

---

## 设计原则

1. **资源导向** — 先定义资源，再定义操作，标准方法优先
2. **一致性** — 相同语义用相同 HTTP 方法、路径模式、状态码
3. **简单优先** — 标准方法能解决的不用自定义方法
4. **避免 PATCH** — `PUT` 全量替换 + `POST + 语义化路径` 已足够清晰

---

## 资源命名

| 规则 | 正确 | 错误 |
|------|------|------|
| 集合用复数名词 | `/users` | `/user` |
| kebab-case | `/user-profiles` | `/userProfiles` |
| 标准方法不放动词 | `POST /users` | `POST /users/create` |
| 最大嵌套 2 层 | `/users/{id}/orders/{oid}` | `/a/{id}/b/{bid}/c/{cid}` |

路径段用清晰简明的英文复数名词，避免 `items`、`objects` 等笼统词。

---

## 标准方法

goddd 采用四个标准方法，不使用 PATCH。

| 方法 | HTTP | 路径 | WrapH 绑定 | 响应体 |
|------|------|------|-----------|--------|
| List | GET | `/resources` | `form` tag → Query | `{"items": [...], "total": N}` |
| Get | GET | `/resources/:id` | `uri` tag → 路由参数 | 资源对象 |
| Create | POST | `/resources` | `json` tag → Body | 创建后的完整资源 |
| Update | PUT | `/resources/:id` | `uri` + `json` tag → Body | 更新后的完整资源 |
| Delete | DELETE | `/resources/:id` | `uri` tag | 空 |

**补充说明**：

- **Delete 幂等**：资源已删除时仍返回成功

```go
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
    Name string `form:"name" binding:"max=12"`
}

type CreateEntityInput struct {
    Name      string `json:"name" binding:"required,max=50"`
    TenantID  string `json:"-"`
    CreatedBy string `json:"-"`
}
```

---

## 自定义方法

标准方法无法表达时使用。统一用 `POST`（只读查询可用 `GET`）。

| 方法 | 路径示例 | 说明 |
|------|---------|------|
| Cancel | `POST /:id/cancel` | 取消操作 |
| Search | `GET /search` | 复杂搜索 |
| Undelete | `POST /:id/undelete` | 撤销删除 |

优先用标准方法。

---

## 错误处理

**核心规则**：业务正常响应 200，业务出错一律响应 400（默认），只有认证和限流走其它状态码。

项目用 `reason.Error` 体系，`web.Fail` 默认返回 400。只有显式调用 `SetHTTPStatus()` 的 error 才会使用其它状态码：

```go
// 默认 400 — 大多数业务错误
reason.ErrBadRequest.SetMsg("参数不合法")
reason.ErrDB.Withf("查询失败: %s", err)
reason.ErrNotFound.SetMsg("资源未找到")
reason.ErrServer.SetMsg("服务器发生错误")

// 显式 401 — 认证失败
reason.ErrUnauthorizedToken.SetMsg("用户已过期")   // SetHTTPStatus(401)

// 显式 429 — 限流
reason.ErrRateLimit.SetMsg("请求频率过高")          // SetHTTPStatus(429)
```

- `SetMsg()` — 用户可见提示
- `Withf()` — 开发调试 details，`SetRelease()` 后不输出
- `SetHTTPStatus()` — 覆盖默认 400 状态码

错误响应结构：

```json
{"reason": "ErrBadRequest", "msg": "名称不能为空", "details": ["field[name]: 空字符串"], "trace_id": "abc123"}
```

- `reason` — error 标识（如 `ErrBadRequest`、`ErrStore`）
- `msg` — 友好提示
- `details` — 仅开发模式可见（`SetRelease()` 后不输出）
- `trace_id` — 请求追踪 ID，用于日志调用链定位

---

## 分页与过滤

嵌入 `web.PagerFilter` 和 `web.DateFilter`：

```go
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
    Name string `form:"name"`
}
```

请求：`GET /entities?page=1&size=20&sort=-created_at&start_ms=1720000000000`

响应：`{"items": [...], "total": 42}`

- `SortSafelist` 白名单防注入，`-` 降序 / `+` 升序
- `NewPagerFilterMaxSize()` 不分页查询
- `DateFilter` 毫秒时间戳，`StartAt()` / `EndAt()` 获取 `time.Time`

---

## 校验

### 绑定层 — Gin `binding` tag

| tag | 说明 |
|-----|------|
| `binding:"required"` | 必填 |
| `binding:"min=1,max=100"` | 数值范围 |
| `binding:"oneof=active inactive"` | 枚举值 |
| `binding:"email"` | 邮箱格式 |
| `binding:"max=255"` | 最大长度 |

绑定失败 `WrapH` 自动返回 400，定位到具体字段。

### 业务层 — `web.Validator`

```go
v := web.NewValidator()
v.Check(in.Name != "", "name", "名称不能为空")
if !v.Valid() {
    return nil, reason.ErrBadRequest.With(v.List()...)
}
```

分工：`binding` 管格式，`Validator` 管业务（唯一、存在、权限、状态）。



---

## 限流

| 中间件 | 粒度 |
|--------|------|
| `web.RateLimiter(r, b)` | 全局 |
| `web.IPRateLimiterForGin(r, b)` | 按 IP |
| `web.IDRateLimiter(r, b, ttl)` | 按用户 ID |

被限流返回 `429`，带 `Retry-After` 头。

---

## 路由注册模板

```go
func registerEntity(r gin.IRouter, api EntityAPI, handler ...gin.HandlerFunc) {
    g := r.Group("/entities", handler...)
    g.GET("", web.WrapH(api.listEntities))
    g.POST("", web.WrapH(api.createEntity))
    g.GET("/:id", web.WrapH(api.getEntity))
    g.PUT("/:id", web.WrapH(api.updateEntity))
    g.DELETE("/:id", web.WrapH(api.deleteEntity))
    g.PUT("/sort", web.WrapH(api.sortEntities))
}
```

---

## 参考

- [Google API 设计指南](https://google-cloud.gitbook.io/api-design-guide)
- [Google API 设计指南 — 标准方法](https://google-cloud.gitbook.io/api-design-guide/standard_methods)
- [Google API 设计指南 — 自定义方法](https://google-cloud.gitbook.io/api-design-guide/custom_methods)
