# Web 工具函数完整参考

`github.com/ixugo/goddd/pkg/web` 包提供 HTTP 请求处理、响应封装、鉴权、日志、限流、SSE、缓存等开发基础设施。

---

## 目录

1. [请求处理与路由包装](#请求处理与路由包装)
2. [分页与日期过滤](#分页与日期过滤)
3. [响应处理](#响应处理)
4. [错误处理](#错误处理)
5. [Context 扩展](#context-扩展)
6. [JWT 鉴权](#jwt-鉴权)
7. [日志中间件](#日志中间件)
8. [限流中间件](#限流中间件)
9. [SSE（Server-Sent Events）](#sseserver-sent-events)
10. [参数校验](#参数校验)

---

## 请求处理与路由包装

### WrapH — 核心路由包装函数

```go
func WrapH[I, O any](fn func(*gin.Context, *I) (O, error)) gin.HandlerFunc
```

将 `func(*gin.Context, *Input) (Output, error)` 包装为 `gin.HandlerFunc`，自动完成：
- POST/PUT/PATCH → `ContentLength > 0` 时按 Content-Type 绑定 Body
- DELETE → `ContentLength > 0` 时绑定 Body，否则绑定 Query
- GET → 绑定 URL Query（`form` tag）
- 路由路径参数 → 自动绑定 `uri` tag
- 错误自动转为统一 HTTP 响应
- 入参第二个参数必须是指针，`*struct{}` 表示无参数

```go
router.GET("/users", web.WrapH(api.findUsers))
router.POST("/users", web.WrapH(api.addUser))
```

#### 文件上传（multipart form）

使用 WrapH 自动绑定上传参数时，将 `*multipart.FileHeader` 与其他 `form` 字段放在同一个输入结构体中。`*struct{}` 是零大小类型，会跳过 WrapH 的自动绑定。

处理上传时检查文件是否存在、请求大小与内容格式；打开文件、解析和关闭资源的错误按项目约定处理。已有手动读取请求的接口无需仅为风格统一而重写。

### WrapHs — 带中间件的路由包装

```go
func WrapHs[I, O any](fn func(*gin.Context, *I) (O, error), mid ...gin.HandlerFunc) []gin.HandlerFunc
```

同 WrapH，附加前置中间件。返回 `[]gin.HandlerFunc`，用于 `r.GET("/path", web.WrapHs(fn, mid1, mid2)...)`

### CustomMethods — 自定义方法路由

```go
func CustomMethods(g gin.IRouter, relativePath string, data map[string]func(*gin.Context))
```

支持 `/:name/sound:muted` 等自定义方法路由（如 Google API 设计规范中的自定义方法）。

---

## 分页与日期过滤

### PagerFilter — 分页参数

```go
type PagerFilter struct {
    Page         int      `form:"page"`
    Size         int      `form:"size"`
    Sort         string   `form:"sort"`
    SortSafelist []string `json:"-"` // 允许的排序字段白名单
}
```

方法：

| 方法 | 说明 |
|------|------|
| `Offset() int` | 计算偏移量 `(Page-1)*Size`，Page < 1 自动修正为 1 |
| `Limit() int` | 每页数量，限制在 1~10000 范围 |
| `SortColumn() (string, bool)` | 去 `-` 前缀后按白名单校验排序列（safelist 只需定义 `"id"` 无需 `"-id"`） |
| `SortDirection() string` | 返回 "ASC" 或 "DESC"（带 `-` 前缀返回 "DESC"） |
| `MustSortColumn() string` | 返回排序列+方向（如 `"created_at DESC"`），不匹配白名单返回空字符串 |

### NewPagerFilterMaxSize

```go
func NewPagerFilterMaxSize() PagerFilter
```

创建 `Size=99999` 的参数，但 `Limit()` 仍返回 `10000`，并不取消分页，也不保证取回全部数据。全量处理应使用项目已有的分页或游标遍历。

`Offset()` 使用原始 `Size` 计算，`Limit()` 的截断不会回写 `Size`。面向请求时应先校验并统一有效页大小，再计算偏移和查询数量，避免页间漏数据。排序使用 `name` 表示升序、`-name` 表示降序；`SortColumn()` 只去掉 `-`，不会去掉 `+`。

### DateFilter — 日期范围过滤

```go
type DateFilter struct {
    StartMs int64 `form:"start_ms"` // 开始毫秒时间戳
    EndMs   int64 `form:"end_ms"`   // 结束毫秒时间戳
}
```

方法：

| 方法 | 说明 |
|------|------|
| `StartAt() time.Time` | 毫秒时间戳转 time.Time |
| `EndAt() time.Time` | 毫秒时间戳转 time.Time |
| `DefaultStartAt(date time.Time) time.Time` | 无效时返回默认值 |
| `DefaultEndAt(date time.Time) time.Time` | 无效时返回默认值 |

---

## 响应处理

### PageOutput[T] — 分页响应

```go
type PageOutput[T any] struct {
    Items []T   `json:"items"`
    Total int64 `json:"total"`
}
```

### ScrollPageOutput[T] — 滚动分页响应

```go
type ScrollPageOutput[T any] struct {
    Items []T    `json:"items"`
    Next  string `json:"next"` // 下一页游标
}
```

### Success / Fail

```go
func Success(c HTTPContext, bean any)
func Fail(c ResponseWriter, err error, fn ...WithData)
func AbortWithStatusJSON(c ResponseWriter, err error, fn ...WithData)
```

---

## 错误处理

WrapH 将处理函数返回的错误交给 `web.Fail`。错误类型、默认 HTTP 状态码和修饰方法见 [api-design-patterns.md](api-design-patterns.md#错误处理与-reason-规范)。

`web.SetRelease()` 隐藏响应中的调试 details，`web.SetDebug()` 开启 details，`web.IsRelease()` 查询当前模式。

---

## Context 扩展

### Context 接口

```go
type Context interface {
    context.Context
    Request() *http.Request
    GetBaseURL() string
    GetScheme() string
    GetHost() string
    BaseURLJoin(...string) string
}
```

### WithContext 与 URL 工具

```go
func WithContext(r *http.Request) Context                      // 包装 Request 为 web.Context
func GetBaseURL(req *http.Request) string                      // 提取 scheme://host
func BaseURLJoin(req *http.Request, paths ...string) string    // 拼接 base URL 与子路径
func GetHost(req *http.Request) string                         // 提取 host
func GetScheme(req *http.Request) string                       // 提取 http/https
func XForwardedPrefix(req *http.Request, path string) string   // 处理反向代理前缀
```

透传前提及标准 context 再包装限制见 [with-context.md](with-context.md)。

### TraceID

```go
func TraceID(ctx context.Context) (string, bool)   // 获取追踪 ID
func MustTraceID(ctx context.Context) string        // 获取追踪 ID，不存在 panic
func SetTraceID(ctx *gin.Context, id string)        // 设置追踪 ID
```

---

## JWT 鉴权

```go
// 创建 Token
data := web.NewClaimsData().
    SetUserID(1).
    SetUsername("admin").
    SetRoleID(1).
    SetLevel(1).
    Set("tenant_id", "t001")

token, err := web.NewToken(data, secret,
    web.WithExpires(24 * time.Hour),
    web.WithIssuer("goddd"),
)

// 中间件鉴权
r.Use(web.AuthMiddleware(secret))
r.Use(web.AuthLevel(2))

// 从上下文读取
uid := web.GetUID(c)
username := web.GetUsername(c)
roleID := web.GetRoleID(c)
level := web.GetLevel(c)
tokenStr := web.GetToken(c)
```

---

## 日志中间件

```go
r.Use(web.Logger(
    web.IgnorePrefix("/health", "/metrics"),
    web.IgnoreMethod("OPTIONS"),
))

// 记录 body（debug 调试）
r.Use(web.LoggerWithBody(1024, web.IgnorePrefix("/upload")))

// 慢请求耗时警告
r.Use(web.LoggerWithUseTime(time.Second, web.IgnorePrefix("/health")))
```

---

## 限流中间件

```go
// 全局限流
r.Use(web.RateLimiter(100, 200))

// 按 IP 限流
r.Use(web.IPRateLimiterForGin(10, 20))

// 按 ID 限流
check := web.IDRateLimiter(1, 5, time.Minute)
if !check(userID) {
    // 触发限流
}
```

---

## SSE（Server-Sent Events）

```go
sse := web.NewSSE(100, 30*time.Second)

sse.Publish(web.Event{
    ID:    "1",
    Event: "progress",
    Data:  []byte(`{"percent": 50}`),
})

sse.Close()
sse.Stop()
```

---

## 参数校验

```go
v := web.NewValidator()
v.Check(len(name) > 0, "name", "名称不能为空")
v.Check(age >= 18, "age", "年龄不能小于 18")

if !v.Valid() {
    return nil, reason.ErrBadRequest.With(v.List()...)
}
```
