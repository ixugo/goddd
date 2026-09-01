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
9. [缓存与 ETag](#缓存与-etag)
10. [SSE（Server-Sent Events）](#sse)
11. [参数校验](#参数校验)
12. [性能分析](#性能分析)
13. [其他工具](#其他工具)

---

## 请求处理与路由包装

### WrapH — 核心路由包装函数

```go
func WrapH[I, O any](fn func(*gin.Context, *I) (O, error)) gin.HandlerFunc
```

将 `func(*gin.Context, *Input) (Output, error)` 包装为 `gin.HandlerFunc`，自动完成：
- POST/PUT/DELETE/PATCH → 绑定 Request Body（`json` tag 或 multipart form）
- GET → 绑定 URL Query（`form` tag）
- 路由路径参数 → 自动绑定 `uri` tag
- 错误自动转为统一 HTTP 响应
- 入参第二个参数必须是指针，`*struct{}` 表示无参数

```go
router.GET("/users", web.WrapH(api.findUsers))
router.POST("/users", web.WrapH(api.addUser))
```

#### 文件上传（multipart form）

文件上传接口（如批量导入）禁止使用 `*struct{}` 入参再手动调用 `c.Request.FormFile("file")`。正确做法是将文件和普通字段写入同一个 in 结构体：

```go
type ImportGreetsInput struct {
    AccessKey string                `form:"access_key" binding:"max=64"`
    File      *multipart.FileHeader `form:"file"`
}

func (a GreetAPI) importGreets(c *gin.Context, in *greet.ImportGreetsInput) (*greet.ImportGreetsResult, error) {
    if in.File == nil {
        return nil, reason.ErrBadRequest.WithMsg("file 参数缺失")
    }
    file, err := in.File.Open()
    if err != nil {
        return nil, reason.ErrBadRequest.WithMsg("读取文件失败")
    }
    defer file.Close()
    // 后续解析 file ...
    return &greet.ImportGreetsResult{}, nil
}
```

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

创建 `Size=99999` 的分页，用于"全量查询不分页"场景。

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

WrapH 内部自动捕获错误，Core 层返回实现了 `reason.ErrorInfoer` 的错误类型（如 `reason.CustomError`）：

```go
reason.ErrBadRequest.WithMsg("参数不合法")              // → 400
reason.ErrNotFound.WithMsg("资源未找到")                // → 400
reason.ErrUnauthorized.WithMsg("用户未登录")           // → 401
reason.ErrPermissionDenied.WithMsg("权限不足")          // → 403
reason.ErrTooManyRequests.WithMsg("请求频率过高")        // → 429
reason.ErrDB.Withf("查询失败: %s", err)                // → 500
reason.ErrServer.WithMsg("服务器发生错误")               // → 500
```

- `WithMsg()`：设置面向用户的友好提示
- `Withf()`：追加 details 信息供开发者排查
- `WithHTTPStatus()`：覆盖默认状态码

环境切换：

```go
web.SetRelease() // 生产环境，details 不输出
web.SetDebug()   // 开发环境，输出 details
web.IsRelease()  // 检查是否生产环境
```

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
