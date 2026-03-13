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
- POST/PUT/DELETE/PATCH → 绑定 Request Body（`json` tag）
- GET → 绑定 URL Query（`form` tag）
- 错误自动转为 HTTP 响应
- 入参第二个参数必须是指针，`*struct{}` 表示无参数

```go
// 使用示例
router.GET("/users", web.WrapH(api.findUsers))
router.POST("/users", web.WrapH(api.addUser))
```

### WrapHs — 带中间件的路由包装

```go
func WrapHs[I, O any](fn func(*gin.Context, *I) (O, error), mid ...gin.HandlerFunc) []gin.HandlerFunc
```

同 WrapH，额外前置中间件。返回 `[]gin.HandlerFunc`，用于 `r.GET("/path", web.WrapHs(fn, mid1, mid2)...)`

### CustomMethods — 自定义方法路由

```go
func CustomMethods(g gin.IRouter, relativePath string, data map[string]func(*gin.Context))
```

支持 `/:name/sound:muted` 等自定义方法路由。

---

## 分页与日期过滤

### PagerFilter — 分页参数

```go
type PagerFilter struct {
    Page         int      `form:"page"`
    Size         int      `form:"size"`
    Sort         string   `form:"sort"`
    SortSafelist []string `form:"-"`    // 允许的排序字段白名单
}
```

方法：

| 方法 | 说明 |
|------|------|
| `Offset() int` | 计算偏移量 `(Page-1)*Size` |
| `Limit() int` | 每页数量，限制 1~10000 |
| `SortColumn() (string, bool)` | 按白名单校验排序列 |
| `SortDirection() string` | 返回 ASC 或 DESC |
| `MustSortColumn() string` | 返回排序列+方向，不校验白名单 |

### NewPagerFilterMaxSize

```go
func NewPagerFilterMaxSize() PagerFilter
```

创建 `Size=99999` 的分页，用于"全量查询不分页"场景。

### DateFilter — 日期范围过滤

```go
type DateFilter struct {
    StartMs int64 `form:"start_ms"`   // 开始毫秒时间戳
    EndMs   int64 `form:"end_ms"`     // 结束毫秒时间戳
}
```

方法：

| 方法 | 说明 |
|------|------|
| `StartAt() time.Time` | 毫秒时间戳转 time.Time |
| `EndAt() time.Time` | 毫秒时间戳转 time.Time |
| `DefaultStartAt(date time.Time) time.Time` | 无效时返回默认值 |
| `DefaultEndAt(date time.Time) time.Time` | 无效时返回默认值 |

```go
// 使用示例
if in.StartMs > 0 && in.EndMs > 0 {
    query.Where("created_at >= ? AND created_at <= ?", in.StartAt(), in.EndAt())
}
```

### 辅助函数

```go
func Limit(v, minV, maxV int) int    // 将 v 限制在 [minV, maxV]
func Offset(page, size int) int       // 计算分页偏移量
```

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
    Next  string `json:"next"`   // 下一页游标
}
```

### Success / Fail

```go
func Success(c HTTPContext, bean any)                           // 200 JSON 响应
func Fail(c ResponseWriter, err error, fn ...WithData)         // 错误 JSON 响应
func AbortWithStatusJSON(c ResponseWriter, err error, fn ...WithData)  // 中止并返回错误
```

### ResponseMsg

```go
type ResponseMsg struct {
    Msg string `json:"msg"`
}
```

通用消息响应。

---

## 错误处理

WrapH 内部自动处理错误，Core 层返回 `reason.Error` 类型：

```go
reason.ErrBadRequest.SetMsg("参数不合法")          // → 400
reason.ErrUnauthorized.SetMsg("未登录")            // → 401
reason.ErrDB.Withf("查询失败: %s", err)            // → 500
reason.ErrServer.Withf("err[%s] detail...", err)   // → 500
```

- `SetMsg()` 设置给用户看的友好提示
- `Withf()` 写入 details 给开发者（生产环境不输出）

环境切换：

```go
web.SetRelease()   // 生产环境，details 不输出
web.SetDebug()     // 开发环境，输出 details
web.IsRelease()    // 检查是否生产环境
```

### HandlerResponseMsg

```go
func HandlerResponseMsg(resp http.Response) error
```

解析 HTTP 响应，非 200 时返回错误。

### HanddleJSONErr

```go
func HanddleJSONErr(err error) error
```

将 JSON 解析错误转为可读错误信息。

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

### WithContext

```go
func WithContext(r *http.Request) Context
```

将 `*http.Request` 包装为 `web.Context`，下游通过类型断言获取：

```go
if wc, ok := ctx.(web.Context); ok {
    baseURL := wc.GetBaseURL()
}
```

### URL 工具

```go
func GetBaseURL(req *http.Request) string      // 提取 scheme://host
func GetHost(req *http.Request) string         // 提取 host
func GetScheme(req *http.Request) string       // 提取 http/https
func XForwardedPrefix(req *http.Request, path string) string  // 处理反向代理前缀
```

### TraceID

```go
func TraceID(ctx context.Context) (string, bool)   // 获取追踪 ID
func MustTraceID(ctx context.Context) string        // 获取追踪 ID，不存在 panic
func SetTraceID(ctx *gin.Context, id string)        // 设置追踪 ID
```

---

## JWT 鉴权

### 创建 Token

```go
data := web.NewClaimsData().
    SetUserID(1).
    SetUsername("admin").
    SetRoleID(1).
    SetLevel(1).
    Set("tenant_id", "t001")

token, err := web.NewToken(data, secret,
    web.WithExpires(24 * time.Hour),
    web.WithIssuer("myapp"),
)
```

### 鉴权中间件

```go
r.Use(web.AuthMiddleware(secret))          // 基础 JWT 鉴权
r.Use(web.AuthLevel(2))                    // 等级鉴权（等级越小权限越大）
```

### 从上下文获取用户信息

```go
uid := web.GetUID(c)            // 用户 ID
username := web.GetUsername(c)  // 用户名
roleID := web.GetRoleID(c)     // 角色 ID
level := web.GetLevel(c)       // 权限等级
token := web.GetToken(c)       // token 字符串
val := web.GetInt(c, "key")    // 自定义 int 值
```

### 解析 Token

```go
claims, err := web.ParseToken(tokenString, secret)
```

---

## 日志中间件

### Logger — 基础请求日志

```go
r.Use(web.Logger(
    web.IgnorePrefix("/health", "/debug"),
    web.IgnoreMethod("OPTIONS"),
    web.IgnorePath("/metrics"),
))
```

### LoggerWithBody — 记录请求体和响应体

```go
r.Use(web.LoggerWithBody(1024,  // 限制 body 大小
    web.IgnorePrefix("/upload"),
))
```

### LoggerWithUseTime — 耗时记录

```go
r.Use(web.LoggerWithUseTime(time.Second,  // 超过 1s 打 warn
    web.IgnorePrefix("/health"),
))
```

### 忽略选项

| 函数 | 说明 |
|------|------|
| `IgnoreBool(v bool)` | 固定布尔值忽略 |
| `IgnoreMethod(method string)` | 忽略指定 HTTP 方法 |
| `IgnorePrefix(prefix ...string)` | 忽略路径前缀 |
| `IgnorePath(path ...string)` | 忽略完整路径 |
| `IgoreContains(substrs ...string)` | 忽略路径含子串的请求 |

---

## 限流中间件

```go
// 全局限流：每秒 100 请求，突发 200
r.Use(web.RateLimiter(100, 200))

// 按 IP 限流：每秒 10 请求，突发 20
r.Use(web.IPRateLimiterForGin(10, 20))

// 按 ID 限流：返回检查函数
check := web.IDRateLimiter(1, 5, time.Minute)
if !check(userID) {
    // 触发限流
}

// 请求体大小限制
r.Use(web.LimitContentLength(10 * 1024 * 1024))  // 10MB
```

---

## 缓存与 ETag

```go
// Cache-Control: max-age=3600
r.Use(web.CacheControlMaxAge(3600))

// ETag 自动计算和 304 响应
r.Use(web.EtagHandler(
    web.IgnorePrefix("/api"),
))
```

---

## SSE

### 创建和发布

```go
sse := web.NewSSE(100, 30*time.Second)

sse.Publish(web.Event{
    ID:    "1",
    Event: "progress",
    Data:  []byte(`{"percent": 50}`),
})

sse.Close()
```

### 作为 HTTP Handler

```go
r.GET("/events", func(c *gin.Context) {
    sse.ServeHTTP(c.Writer, c.Request)
})
```

### 分块发送

```go
ch := make(chan web.Chunk)
go web.SendChunk(ch, c)

ch <- web.Chunk{Total: 100, Current: 50, Success: 48, Failure: 2}
```

### NewEventMessage

```go
msg := web.NewEventMessage("update", map[string]any{"id": 1, "status": "done"})
```

---

## 参数校验

```go
v := web.NewValidator()
v.Check(len(name) > 0, "name", "名称不能为空")
v.Check(age >= 18, "age", "年龄不能小于 18")

if !v.Valid() {
    // v.List() 返回错误列表
    // v.Result() 返回 (bool, []string)
}

// 也可以手动添加错误
v.AddError("email", "邮箱格式不正确")
```

---

## 性能分析

```go
// 注册 pprof 路由，仅允许指定 IP 访问
ips := []string{"127.0.0.1", "192.168.1.100"}
web.SetupPProf(r, &ips)

// 启用互斥锁采样
web.SetupMutexProfile(1)
```

---

## 其他工具

### Recover — panic 恢复

```go
r.Use(web.Recover())
```

### SetDeadline — 超时控制

```go
r.Use(web.SetDeadline(30 * time.Second))
```

### Metrics — 请求指标

```go
r.Use(web.Metrics())
web.CountGoroutines(10*time.Second, 100)  // 记录 goroutine 数量
```

### RecordResponse — 记录响应体

```go
r.Use(web.RecordResponse())
```

### AddHead — 添加 Vary 头

```go
r.Use(web.AddHead())  // Vary: Authorization
```
