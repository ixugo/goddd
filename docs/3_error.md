# 一行 return err，搞定所有错误响应

为什么 goddd 的接口只需要 `return nil, err` 就能自动返回结构化的 JSON 错误响应？为什么不用手写 `ctx.JSON(500, ...)`，前端却能拿到机器可读的错误码、用户可读的提示、开发者可追溯的详情？

答案在 `pkg/reason`——**用字符串 reason 代替数字错误码，同时保留面向用户的 msg 和面向开发者的 details**。调用方能根据 reason 做分支判断，用户能看到友好的提示，开发者又能拿到足够的排查信息。

> Go 谚语：
>
> **"Don't just check errors, handle them gracefully."**
>
> 错误不仅要捕获，还要处理好、传清楚。

## 1. 传统方案的问题

常见的错误处理方式大致有两类：

**直接返回底层错误：**

```go
user, err := db.FindUser(id)
if err != nil {
    ctx.JSON(500, gin.H{"error": err.Error()})
    return
}
```

这种方式会泄露内部信息，比如数据库表名、SQL 细节等。

**自定义数字错误码：**

```go
ctx.JSON(200, gin.H{"code": 1001, "msg": "数据库查询失败"})
```

数字没有语义，过段时间连定义者自己都忘了 1001 是什么意思。

## 2. goddd 的错误结构

goddd 把错误拆成几个字段，定义在 [`pkg/reason/reason.go`](https://github.com/ixugo/goddd/blob/ddae5ac9ed1af111a7b2144a3bbba50df8edb02d/pkg/reason/reason.go#L48) 中：

```go
type Error struct {
    Reason     string   `json:"reason"`
    Msg        string   `json:"msg"`
    Details    []string `json:"details"`
    HTTPStatus int      `json:"-"`
    Cause      error    `json:"-"`
}
```

- **`reason`**：英文大驼峰命名，如 `ErrBadRequest`、`ErrUserNotFound`，给程序识别用。
- **`msg`**：中文或本地语言，直接展示给用户看。
- **`details`**：给开发者看的补充信息，比如具体哪个字段校验失败。
- **`HTTPStatus`**：映射到 HTTP 响应状态码，默认 400。
- **`Cause`**：底层原因错误，不序列化到 JSON，仅供 `errors.Is/As` 链路解包使用。

创建错误时使用 `reason.NewError`：

```go
var ErrBadRequest = reason.NewError("ErrBadRequest", "请求参数有误")
var ErrUserNotFound = reason.NewError("ErrUserNotFound", "用户不存在")
```

`NewError` 内部会检查 reason 是否重复，重复会 panic——在启动阶段就暴露冲突。reason 全局唯一，即使不同的包也不允许重复，因为它是面向调用方的机器标识。

**为什么用字符串而非数字？** 核心原因是可读性。`ErrUserNotFound` 天然表达含义，代码里一搜就能定位；`10023` 要先查文档。跨团队、前端分支、监控聚合、国际化等场景，字符串都能胜任——Stripe 用 `card_declined`，AWS IAM 用 `InvalidAction`，Google API 用 `NOT_FOUND`。HTTP 状态码仍然是数字（传输层语义），但业务错误码用字符串更清晰。

## 3. 在业务代码中使用

在 Core 层链式调用：

```go
func (c *Core) GetUser(id int64) (*UserOutput, error) {
    if id <= 0 {
        return nil, reason.ErrBadRequest.With("用户 ID 必须大于 0")
    }

    user, err := c.store.GetUser(id)
    if err != nil {
        return nil, reason.ErrDB.WithCause(err).With("查询用户失败")
    }
    if user == nil {
        return nil, reason.ErrUserNotFound.WithMsg("未找到该用户")
    }

    return user, nil
}
```

链式方法一览：

| 方法 | 作用 | 示例 |
|---|---|---|
| `With(args...)` | 追加 details（开发者排查信息） | `.With("用户 ID 必须大于 0")` |
| `Withf(format, args...)` | 格式化追加 details | `.Withf("ID=%d 不合法", id)` |
| `WithCause(err)` | 包裹底层错误，保留 `errors.Is/As` 链路；多次调用时并列累加 | `.WithCause(err)` |
| `WithMsg(s)` | 覆盖面向用户的提示信息 | `.WithMsg("未找到该用户")` |
| `WithHTTPStatus(code)` | 覆盖 HTTP 响应状态码 | `.WithHTTPStatus(401)` |

所有链式方法都返回新对象，不修改原错误——预定义的错误变量始终是安全的。

结合上一篇的 `web.WrapH`，错误会自动被 `web.Fail` 转成统一响应：

```
客户端 POST /api/users/0
  → WrapH 绑定参数，调用 Core.GetUser(0)
    → Core 返回 reason.ErrBadRequest.With("用户 ID 必须大于 0")
  → WrapH 调用 web.Fail
    → Fail 通过 reason.ErrorInfoer 接口提取 reason/msg/details/HTTPStatus
  → 客户端收到 HTTP 400：

{
    "reason": "ErrBadRequest",
    "msg": "请求参数有误",
    "details": ["用户 ID 必须大于 0"]
}
```

Core 返回 `nil` error 时，`WrapH` 调用 `web.Success` 直接输出业务数据（HTTP 200）。

## 4. 与标准库 errors 的关系

`reason.Error` 完全兼容 Go 标准库，实现了三个关键接口：

**`Unwrap() error`**：返回通过 `WithCause` 包裹的底层错误。

```go
cause := errors.New("connection refused")
err := reason.ErrDB.WithCause(cause).With("查询用户失败")
errors.Is(err, cause) // true
```

多次调用 `WithCause` 时，底层错误通过 `errors.Join` 并列累加，不会覆盖前一个——这在逐层包裹错误时尤为重要，每一层的根因都不会丢：

```go
cause1 := errors.New("connection refused")
cause2 := errors.New("timeout")
err := reason.ErrDB.WithCause(cause1).WithCause(cause2)
errors.Is(err, cause1) // true
errors.Is(err, cause2) // true
```

**`Is(target error) bool`**：按 `Reason` 字符串比较，而非指针比较。即使经过 `With/WithMsg` 产生了新对象，Reason 相同就匹配：

```go
e2 := reason.ErrBadRequest.With("参数错误")
e3 := fmt.Errorf("包装: %w", e2)
errors.Is(e3, reason.ErrBadRequest) // true
```

**`As(target any) bool`**：支持将错误提取为 `*reason.Error`：

```go
var e *reason.Error
if errors.As(err, &e) {
    fmt.Println(e.GetReason(), e.GetMessage())
}
```

## 5. 预定义错误

goddd 内置了一组常用错误（参考 [Google API 错误规范](https://cloud.google.com/apis/design/errors)），按 HTTP 状态码分组：

**客户端错误（默认 400）：**

| 变量 | Reason | HTTP | 含义 |
|---|---|---|---|
| `ErrBadRequest` | `ErrBadRequest` | 400 | 参数无效 |
| `ErrNotFound` | `ErrNotFound` | 400 | 资源未找到 |
| `ErrConflict` | `ErrConflict` | 400 | 操作冲突 |
| `ErrAborted` | `ErrAborted` | 400 | 操作被中止 |
| `ErrJSON` | `ErrJSON` | 400 | JSON 编解码出错 |
| `ErrUnauthorized` | `ErrUnauthorized` | 401 | 未登录或凭证已过期 |
| `ErrPermissionDenied` | `ErrPermissionDenied` | 403 | 权限不足 |
| `ErrFileTooLarge` | `ErrFileTooLarge` | 413 | 文件大小超出限制 |
| `ErrContentTooLarge` | `ErrContentTooLarge` | 413 | 请求体过大 |
| `ErrTooManyRequests` | `ErrTooManyRequests` | 429 | 请求频率过高 |

**服务端错误（默认 500）：**

| 变量 | Reason | HTTP | 含义 |
|---|---|---|---|
| `ErrInternal` | `ErrInternal` | 500 | 服务器内部错误 |
| `ErrDB` | `ErrStore` | 500 | 数据层错误 |
| `ErrServer` | `ErrServer` | 500 | 服务器错误 |
| `ErrUnimplemented` | `ErrUnimplemented` | 501 | 功能尚未实现 |
| `ErrServiceUnavailable` | `ErrServiceUnavailable` | 503 | 服务暂时不可用 |
| `ErrTimeout` | `ErrTimeout` | 504 | 请求超时 |

业务项目根据实际需求，用 `NewError` 定义领域专属的错误：

```go
var ErrUserNotFound      = reason.NewError("ErrUserNotFound", "用户不存在")
var ErrOrderExpired      = reason.NewError("ErrOrderExpired", "订单已过期")
var ErrStockInsufficient = reason.NewError("ErrStockInsufficient", "库存不足")
```

命名建议：`Err` 前缀 + 领域名 + 具体错误（如 `ErrUserNotFound`），确保跨模块不冲突。

## 6. 生产环境隐藏 details

开发时 details 越详细越好，但生产环境可能包含敏感信息。`pkg/web` 提供了开关：

```go
web.SetRelease() // 生产环境调用，错误响应不再输出 details
```

`web.Fail` 会根据这个开关决定是否把 details 放进响应体。默认是 debug 模式，会输出 details。

## 7. 多语言支持

`reason` 天然支持多语言：用 `reason` 字符串作为翻译 key，通过 `WithMsg` 传入翻译后的文本即可。

```go
return nil, reason.ErrNotFound.WithMsg(i18n.T(lang, "ErrNotFound"))
```

`details` 不需要翻译——它记录的是面向开发者的错误详情、解决方案或文档地址，与语言无关。

## 关于 goddd

[goddd](https://github.com/ixugo/goddd) 是一个 AI 驱动的 Go 语言脚手架，基于领域驱动设计（DDD）思想搭建，提供了一套适合中小项目快速启动的工程结构和最佳实践。

如果你想了解更多细节，可以访问：

- 官方文档站点：https://goddd.golang.space/
- GitHub 仓库：https://github.com/ixugo/goddd
