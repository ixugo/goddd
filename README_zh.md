<p align="center">
    <img src="./logo.png#gh-light-mode-only" alt="GoDDD Logo" width="550"/>
    <img src="./logo_dark.png#gh-dark-mode-only" alt="GoDDD Logo" width="550"/>
</p>

<p align="center">
    <a href="https://github.com/ixugo/goddd/releases"><img src="https://img.shields.io/github/v/release/ixugo/goddd?include_prereleases" alt="Version"/></a>
    <a href="https://github.com/ixugo/goddd/blob/master/LICENSE.txt"><img src="https://img.shields.io/dub/l/vibe-d.svg" alt="License"/></a>
	<a href="https://goreportcard.com/report/github.com/ixugo/goddd">
	<img src="https://goreportcard.com/badge/github.com/ixugo/goddd"/>
	</a>
	<a href="https://gin-gonic.com"><img width=30px  src="https://avatars.githubusercontent.com/u/7894478?s=48&v=4" alt="GIN"/></a>
    <a href="https://gorm.io"><img width=70px src="https://gorm.io/gorm.svg" alt="GORM"/></a>

</p>

[English](./README.md) | [简体中文](./README_zh.md)

# 企业 REST API 模板

这是一个专注于 REST API 的完整 CURD 解决方案。

GoDDD 目标是:

+ 整洁架构，适用于中小型项目
+ 提供积木套装，快速开始项目，专注于业务开发
+ 令项目更简单，令研发心情更好

如果你觉得以上描述符合你的需求，那就快速开始吧。

支持[代码自动生成](github.com/ixugo/godddx)

支持[事件总线/事务消息](github.com/ixugo/nsqite)

## 设计说明

传统 MVC 单体架构，随着业务扩展，团队越难以有效开发，新入职的同事也很难去了解这个臃肿的单体。

模块化单体架构具有单体架构的许多优点和缺点，也具有微服务架构的大量优点少数缺点。

将完整业务拆分成多个领域模块，例如 用户领域 / 银行领域 / 商品领域，每个领域都有各自一套完善的

+ API(接口)
+ Core(业务)
+ Store(缓存/持久化存储)

不同的开发人员或团队，可以独立地处理这些领域模块，降低开发新功能而导致的冲突混乱。相比微服务而言，这样拆分模块代码，更小更简洁更易于测试。

当程序超出领域模块规模后，团队能够在需要时轻松地将领域模块提取到微服务中。


## 快速开始

1. Golang 版本 >= 1.23.0
2. `git clone github.com/ixugo/goddd`
3. `cd goddd && go build -o goddd ./cmd/server && ./goddd`
4. 新开一个终端访问 `curl http://localhost:8080/health`

注意: 用编辑器运行时，要指定输出目录为项目根目录


## 引用文章

[Google API Design Guide](https://google-cloud.gitbook.io/api-design-guide)

此项目的最佳实践: https://github.com/gowvp/gb28181

## 目录说明


```bash
.
├── main.go					主函数入口
├── cmd						更多可执行程序
├── configs					配置文件
├── docs					设计文档/用户文档
├── internal					私有业务
│   ├── conf					配置模型
│   ├── core					业务领域
│   │   └── version				实际业务
│   │       └── store
│   │           └── versiondb 		数据库操作
│   ├── data					数据库初始化
│   └── web
│       └── api					RESTful API
└── pkg						依赖库
```


## 项目说明

1. 程序启动强依赖的组件，发生异常时主动 panic，尽快崩溃尽快解决错误。

2. core 为业务领域，包含领域模型，领域业务功能

3. store 为数据库操作模块，需要依赖模型，此处依赖反转 core，避免每一层都定义模型。

4. api 层的入参/出参，可以正向依赖 core 层定义模型，参数模型以 `Input/Output` 来简单区分入参出数。


## 请求入参封装

本项目使用 GIN 作为 web 处理框架，路由函数需要实现 `gin.HandlerFunc`，在实现 API 层函数时，遇到的第一个问题是绑定参数，几乎每个函数都会涉及到反序列化，函数开头都充斥了 `ctx.ShouldBindJSON` 之类的代码。

根据 DRY（Don't Repeat Yourself）设计原则，通过减少重复代码来提高代码的可维护性和可重用性。该项目封装了 `web.WarpH` 其返回 `gin.HandlerFunc`，`web.WarpH` 的参数类似 GRPC，`func(ctx *gin.Context, in *struct{}) (*Output, error)`。

WarpH 内部识别 POST/PUT/DELETE/PATCH 请求则绑定 Request Body，Get 请求则绑定 Request URL params。

入参第二个参数类型必须是指针，使用 `*struct{}` 表示没有参数，不需要绑定。在定义结构体时，尤其要注意结构体的 tag 应该是 `json` 或者 `form`，更多细节参考 GIN 框架参数绑定。

+ `json` 可绑定 request body 参数
+ `form` 可绑定 params 参数

返回值第一个参数是具体的 response body 内容，建议避免使用 any，其类型即可以是值，也可以是指针，赋予了更多灵活性。

当参数在多个位置时，即路由参数/查询参数/请求体参数同时存在，可以实现新的 web.WarpH2 或直接实现 `gin.HandlerFunc`。

以下是两种代码的示例:

```go
func findUser(ctx *gin.Context) {
	var in findUserInput
	if err := ctx.ShouldBindQuery(&in);err!=nil {
		ctx.JSON(...)
		return
	}
	out,err := serviceFunc(in)
	// ....
}
```

```go
func findUsers(ctx *gin.Context, in *Input) (*Output, error) {
	return serviceFunc(in)
}
```

## 响应出参封装

明确的定义出参类型，可以使代码更容易读懂，我希望通过更多细节提升代码的可读性，可维护性。

`web.Warh` 的封装默认是响应 application/json 类型。

在开发过程中，新同事实现 `gin.HandlerFunc` 时更容易遗忘 `return` 语句。使用 `web.WarpH` 能确保不遗落 `return`。

以下是两种代码的示例

```go
func findUsers(ctx *gin.Context) {
	// 可能 out 是从业务层获取的
	// 此时想知道 response body 需要往函数内部找
	out,err := serviceFunc()
	if err != nil {
		ctx.JSON(...)
		return
	}
	ctx.JSON(out)
}
```

```go
func findUsers(ctx *gin.Context, in *Input) (*Output, error) {
	return serviceFunc(in)
}
```

## 错误处理

通过上面的代码了解到，错误是直接 return 的，难倒不担心底层的错误信息暴露给用户吗? 还有错误的 http statusCode 又是多少呢?

其实在 `web.Warn` 中还做了一些事情，比如在绑定过程中出错，可以定位到具体的错误原因，是类型不对? 错在哪个属性上? 比如响应的时候，通过 err 提取出信息，返回对应的 HTTP 状态码，接下来详细介绍错误处理。

`pkg/web` 是 HTTP 相关的处理包，包含中间件，响应，错误处理，鉴权，日志，限流，指标，性能分析，入参校验等等。

我们自定义一个 Error 类型， `reason` 是错误原因，有些第三方 API 也会用 Code。

该项目在设计的时候，考虑到状态码不易读，比如错误 `10020`，请问是什么错误? 所以定义了 `reason`，应该用大驼峰英语简略描述错误原因。那如果就是想用状态码表示呢? 请用 HTTP StatusCode。

msg 应当是开发者母语的错误描述，`reason` 用于程序内部判定，`msg` 用于友好提示给用户。`details` 是错误的扩展，提供给开发者，可以描述错误的解决方案，提供文档，错误的更细节详情，甚至暴露更底层的错误信息。

通常在前后端分离项目中，前端遇到一些错误，都需要询问后端发生了什么情况，通过 `details` 前端可以减少更多提问。

在 `web.WarpH` 的封装中，错误实际是调用的 `web.Fail(err)`，此方法会判断 `reason` 应该返回怎样的 http statusCode，开发者可以在 `pkg/web/error.go` 中 `HTTPCode()` 函数实现更多 http statusCode 扩展，默认提供了 200/400/401 三种状态码。

details 应该仅开发模式可见，`web.SetRelease()` 可以设置为生产发布模式，此时 details 将不会写入 http response body。


core 层导出的函数或 API 层返回的错误，应该返回 reason.Error 类型的错误。

在封装的 web.WarpH 中，会正确记录错误到日志并返回给前端。

```go
func findUser(in *Input)  (*Output,error){
	// 数据库操作发生错误
	if err != nil {
		return nil, reason.ErrDB.SetMsg() // 错误的 respon 类型是 db 层错误，Msg 函数可以更改给用户的友好提示
	}
	// 业务发生错误
	if err != nil {
		return nil, reason.ErrServer.Withf("err[%s] ....",err) // Withf 可以写入 details 给开发者更多提示
	}
}
```


## Makefile

Windows 系统使用 makefile 时，请使用 git bash 终端，不要使用系统默认的 cmd/powershell 终端，否则可能会出现异常情况。

执行 `make` 或 `make help` 来获取更多帮助

在编写 makefile 时，应主动在命令上面增加注释，以 `## <命令>: <描述>` 格式书写，具体参数 Makefile 文件已有命令。其目的是 `make help` 时提供更多信息。

makefile 中提供了一些默认的操作便于快速编写

`make confirm` 用于确认下一步

`make title content=标题`  用于重点突出输出标题

`make info` 获取构建版本相关信息

**makefile 构建的版本号规则说明**

1. 版本号使用 Git tag，格式为 v1.0.0。

2. 如果当前提交没有 tag，找到最近的 tag，计算从该 tag 到当前提交的提交次数。例如，最近的 tag 为 v1.0.1，当前提交距离它有 10 次提交，则版本号为 v1.0.11（v1.0.1 + 10 次提交）。

3. 如果没有任何 tag，则默认版本号为 v0.0.0，后续提交次数作为版本号的次版本号。

## 库如何使用?

### hook.UseCache 临时缓存

old

```go
cache := make(map[string]string)
for i := range 10 {
	v, ok := cache[i]
	if ok {
		// 业务处理
		continue
	}
	v,err := fn()
	if err == nil {
		cache[v.ID] = v
	}
	// 业务处理
}
```

new

```go
	cacheFn :=  hook.UseCache(fn)
	for i := range 10 {
		v,_,err :=  cacheFn(i)
		if err == nil {
			// 业务处理
		}
	}
```

### hook.UseTiming 计算函数花销写日志

old

```go
	now := time.Now()
	// 业务处理夹杂在时间计算中
	if sub :=time.Since(now); sub > time.Second {
		slog.Error("函数名", "cost", cost)
	}else {
		slog.Debug("函数名", "cost", cost)
	}
```

new

```go
	cost := hook.UseTiming(time.Second)
	defer cost()

	// 业务处理
```

**更多 hook 直接看 pkg/hook 源码吧**




## 快速开始

业务说明:

假设我们要做一个版本管理的业务，curd 步骤如下:

在 「internal」-「core」 创建 「version」 目录，创建「model.go」写入领域模型，该模型为数据库表结构映射。

创建「core.go」 写入如下内容

```go
package version

import (
	"fmt"
	"strings"
)

// Storer 依赖反转的数据持久化接口
type Storer interface {
	First(*Version) error
	Add(*Version) error
}

// Core 业务对象
type Core struct {
	Storer    Storer
}

// NewCore 创建业务对象
func NewCore(store Storer) *Core {
	return &Core{
		Storer: store,
	}
}

// IsAutoMigrate 是否需要进行表迁移
// 判断硬编码在代码中的数据库表版本号，与数据库存储的版本号做对比
func (c *Core) IsAutoMigrate(currentVer, remark string) bool {
	var ver Version
	if err := c.Storer.First(&ver); err != nil {
		isMigrate := true
		c.IsMigrate = &isMigrate
		return isMigrate
	}
	isMigrate := compareVersionFunc(currentVer, ver.Version, func(a, b string) bool {
		return a > b
	})
	c.IsMigrate = &isMigrate
	return isMigrate
}

func compareVersionFunc(a, b string, f func(a, b string) bool) bool {
	s1 := versionToStr(a)
	s2 := versionToStr(b)
	if len(s1) != len(s2) {
		return true
	}
	return f(s1, s2)
}

func versionToStr(str string) string {
	var result strings.Builder
	arr := strings.Split(str, ".")
	for _, item := range arr {
		if idx := strings.Index(item, "-"); idx != -1 {
			item = item[0:idx]
		}
		result.WriteString(fmt.Sprintf("%03s", item))
	}
	return result.String()
}
```

创建 「store/versiondb」 目录，创建「db.go」 文件写入

```go
type DB struct {
	db *gorm.DB
}

func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// AutoMigrate 表迁移
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(version.Version),
	); err != nil {
		panic(err)
	}
	return d
}

func (d DB) First(v *version.Version) error {
	return d.db.Order("id DESC").First(v).Error
}

func (d DB) Add(v *version.Version) error {
	return d.db.Create(v).Error
}
```

在 API 层做依赖注入，对 「web/api/provider.go」 写入函数，往 Usecase 中注入业务对象

```go
var ProviderSet = wire.NewSet(
	wire.Struct(new(Usecase), "*"),
	NewHTTPHandler,
	NewVersion,
)

func NewVersion(db *gorm.DB) *version.Core {
	vdb := versiondb.NewDB(db)
	core := version.NewCore(vdb)
	isOK := core.IsAutoMigrate(dbVersion, dbRemark)
	vdb.AutoMigrate(isOK)
	if isOK {
		slog.Info("更新数据库表结构")
		if err := core.RecordVersion(dbVersion, dbRemark); err != nil {
			slog.Error("RecordVersion", "err", err)
		}
	}
	// 其它组件可以调用此变量，判断是否需要表迁移
	orm.EnabledAutoMigrate = isOK
	return core
}
```

在 API 层新建「version.go」文件，写入

```go
// version 业务函数命名空间
type VersionAPI struct {
	ver *version.Core
}

func NewVersionAPI(ver *version.Core) VersionAPI {
	return VersionAPI{ver: ver}
}
// registerVersion 向路由注册业务接口
func registerVersion(r gin.IRouter, verAPI VersionAPI, handler ...gin.HandlerFunc) {
	ver := r.Group("/version", handler...)
	ver.GET("", web.WarpH(verAPI.getVersion))
}

func (v VersionAPI) getVersion(_ *gin.Context, _ *struct{}) (any, error) {
	return gin.H{"msg": "test"}, nil
}
```


## 常见问题

> 为什么不在每一层分别定义模型?

开发效率与解耦的取舍，在代码通俗易懂和效率之间取的平衡。

> 那 api 层参数模型，表映射模型到底应该定义在哪里?

要清楚各层之间的依赖关系，api 直接依赖 core，db 依赖反转 core。故而领域模型定义在 core 中，api 的入参和出参也可以定义在 core，当然 core 层用不上的结构体，定义在 API 层也无妨。

> 为什么 api 层直接依赖 core 层，而不是依赖接口?

接口的目的是为了解耦，在实际开发过程中，更多是替换 api 层，而不是替换 core 层。

API 只做参数获取，返回响应参数，只做最少的事情，方便从 HTTP 快速过度的 GRPC。

面向未来设计，面向当下编程，先提高搬砖效率，等未来需要的那天会有更好的方式重构。

> 为什么 db 依赖反转 core?

数据持久化不是独立的，它为业务而服务。即持久化服务于业务，依赖于业务。

通过依赖反转，业务可以在中间穿插 redis cache 等其它 db 。

> 为什么入参/出参模型以  Input/Output 单词结尾

约定大于配置，类似有些项目以 Request/Response 单词结尾，只是为了有一个统一的，大家都明确的参数。

当然，有可能出参也是入参，你可以定义别名，也可以直接使用。

很多时候，我们都想明确自己在做什么，为什么这样做，这个「常见问题」希望能提供一点解惑思路。

> 如何为 goddd 编写业务插件?

```go
// RegisterVersion 有一些通用的业务，它们被其它业务依赖，属于业务的基层模块，例如表版本控制，字典，验证码，定时任务，用户管理等等。
// 约定以 Register<Core> 方式编写函数，注入 gin 路由，命名空间，中间件三个参数。
// 具体可以参考项目代码
func RegisterVersion(r gin.IRouter, verAPI VersionAPI, handler ...gin.HandlerFunc) {
	ver := r.Group("/version", handler...)
	ver.GET("", web.WarpH(verAPI.getVersion))
}
```

## 表迁移

每次程序启动都执行一遍，太慢了。

所以通过 version 表来控制，是否要进行表迁移操作。

当发现数据库表版本已经是最新时，即不执行。通过修改 api/db.go 文件中 dbVersion 控制版本号。

## 自定义配置目录

默认配置目录为可执行文件同目录下的 configs，也可以指定其它配置目录

`./bin -conf ./configs`


## 项目主要依赖

+ gin
+ gorm
+ slog / zap
+ wire