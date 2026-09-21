# API 设计规范

本页约定用于设计新的 GoDDD API。维护已有接口时，保留其路径、HTTP 方法、状态码、响应结构和查询语义；不要为统一风格修改已上线契约。

---

## 设计原则

1. **资源导向** — 先定义资源，再定义操作，标准方法优先
2. **一致性** — 相同语义用相同 HTTP 方法、路径模式、状态码
3. **简单优先** — 标准方法能解决的不用自定义方法
4. **更新语义明确** — 新接口优先沿用项目惯例；`PUT` 表示完整替换，局部更新应明确字段缺省语义。已有 `PATCH` 接口保持兼容。

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

新资源接口可参考以下五种操作；表中响应是设计建议，不要求迁移已有接口。

| 方法 | HTTP | 路径 | WrapH 绑定 | 响应体 |
|------|------|------|-----------|--------|
| List | GET | `/resources` | `form` tag → Query | `{"items": [...], "total": N}` |
| Get | GET | `/resources/:id` | `uri` tag → 路由参数 | 资源对象 |
| Create | POST | `/resources` | `json` tag → Body | 创建后的完整资源 |
| Update | PUT | `/resources/:id` | `uri` + `json` tag → Body | 更新后的完整资源 |
| Delete | DELETE | `/resources/:id` | `uri` tag | 空或被删实体 |

**补充说明**：
- **Delete 幂等**：资源已删除时仍返回成功

```go
type ListEntityInput struct {
    web.PagerFilter
    web.DateFilter
    Name string `form:"name" binding:"max=64"`
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

---

## 错误处理与 reason 规范

项目使用 `pkg/reason` 体系，`web.Fail` 会自动提取 `CustomError` 中的 HTTP 状态码并映射响应。

### 常用错误变量

| 变量 | 说明 | HTTP 状态码 |
|------|------|------------|
| `reason.ErrBadRequest` | 客户端参数有误 | 400 |
| `reason.ErrNotFound` | 资源未找到 | 400 |
| `reason.ErrConflict` | 资源冲突/操作冲突 | 400 |
| `reason.ErrJSON` / `ErrUsedLogic` | 业务/解析错误 | 400 |
| `reason.ErrUnauthorized` | 未登录或凭证已过期 | 401 |
| `reason.ErrPermissionDenied` | 没有该资源权限 | 403 |
| `reason.ErrFileTooLarge` / `ErrContentTooLarge` | 文件或请求体过大 | 413 |
| `reason.ErrTooManyRequests` | 触发限流频率过高 | 429 |
| `reason.ErrDB` / `ErrServer` / `ErrInternal` | 数据或服务器内部错误 | 500 |
| `reason.ErrTimeout` | 超时 | 504 |

### 错误链式修饰方法（不可变返回副本）

```go
// WithMsg 覆盖面向用户的友好提示
reason.ErrBadRequest.WithMsg("用户名称不能为空")

// Withf 追加开发者调试 details（生产环境 SetRelease 后不输出）
reason.ErrDB.Withf("查询用户失败 id[%d]: %w", id, err)

// WithHTTPStatus 覆盖默认状态码
reason.ErrBadRequest.WithHTTPStatus(422)

// WithCause 携带底层 error 链，支持 errors.Is / errors.As
reason.ErrDB.WithCause(err)
```

错误响应结构体示例：

```json
{
  "reason": "ErrBadRequest",
  "msg": "用户名称不能为空",
  "details": ["field[name]: 长度超出限制"],
  "trace_id": "abc123xyz"
}
```

---

## 分页与过滤

分页、日期过滤和排序字段行为见 [web-toolkit.md](web-toolkit.md#分页与日期过滤)，不要在 API 层另写一套与其不一致的参数解释。

新接口可约定空列表返回 `"items": []`，并在输出边界保证空切片初始化。维护已有接口时保持原来的 `[]` 或 `null` 契约。

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

### 业务层 — `web.Validator`

```go
v := web.NewValidator()
v.Check(in.Name != "", "name", "名称不能为空")
if !v.Valid() {
    return nil, reason.ErrBadRequest.With(v.List()...)
}
```

### 转换层 — 必须处理转换错误

路由参数转数值时，转换错误必须返回 400，严禁忽略错误使用零值。

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
