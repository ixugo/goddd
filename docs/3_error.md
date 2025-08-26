# 用 HTTP 状态码还是自定义状态码? Go 错误处理的优雅解决方案

在开发 REST API 时，你是否遇到过这样的问题：

- 错误信息杂乱无章，难以定位问题
- 用户看到的错误提示晦涩难懂
- 生产环境暴露了敏感信息
- 错误处理代码重复且难以维护

本文将介绍一种优雅的错误处理解决方案，让你的代码更加清晰、可维护，同时提供更好的用户体验。

## 错误处理的痛点

在传统的错误处理方式中，我们常常会遇到以下问题：

1. 错误信息不统一
   - 有的使用数字状态码
   - 有的使用字符串描述
   - 有的直接返回底层错误

2. 错误处理代码重复
   - 每个接口都要写错误处理逻辑
   - 日志记录分散在各处
   - HTTP 状态码映射混乱

3. 用户体验差
   - 错误提示不友好
   - 缺乏上下文信息
   - 难以定位问题

## 传统错误处理方案

让我们先看看传统的错误处理方式：

```go
// 方式一：直接返回错误
func findUser(ctx *gin.Context) {
    user, err := db.FindUser()
    if err != nil {
        ctx.JSON(500, gin.H{"error": err.Error()})
        return
    }
    ctx.JSON(200, user)
}

// 方式二：使用状态码
func findUser(ctx *gin.Context) {
    user, err := db.FindUser()
    if err != nil {
        ctx.JSON(500, gin.H{
            "code": 1001,
            "msg": "数据库查询失败"
        })
        return
    }
    ctx.JSON(200, user)
}
```

这些方案虽然简单直接，但存在一些明显的缺陷：

1. 安全性问题
   - 方式一直接将底层错误暴露给用户，可能泄露敏感信息
   - 数据库错误可能包含表结构、SQL 语句等内部信息

2. 沟通复杂性
   - 错误状态码 400 是 http 状态码还是自定义状态码?
   - 必须对响应体反序列化，才能知道状态
   - 重复定义，http 状态码 200 与自定义状态码 0 都表示成功

3. 可维护性差
   - 数字错误码（如 1001）缺乏语义，难以理解
   - 开发者需要查阅文档才能理解错误含义
   - 错误码定义者也可能忘记具体含义

## 优雅的错误处理方案

让我们先看看优雅的错误处理方案是如何使用的：

```go
var ErrBadRequest = reason.NewError("ErrBadRequest", "请求参数有误")

func (u *UserAPI) getUser(ctx *gin.Context, _ *struct{}) (*user.UserOutput, error) {
    return u.core.GetUser(in.ID)
}

// package user
func (u *Core) GetUser(id int64) (*UserOutput, error) {
	// 参数校验
	if err != nil {
		return nil, ErrBadRequest.With(err.Error(), "xx 参数应在 10~100 之间")
	}
	// 正确处理逻辑...
}
```

还记得上一篇文章提到的 `web.WrapH` 函数吗? 其响应错误实际是调用的 `web.Fail(err)`，此方法会判断错误是否是 `reason.Error` 类型，如果是，则按照其定义的 http 状态码，reason, msg 等信息返回给客户端。

类似

HTTP Status Code: 400 (默认所有错误都是 400)
```json
{
    "reason": "用于程序识别的错误",
    "msg": "告诉用户的错误信息描述",
    "details":[
      "某字段传输有误",
      "你可以这样修复",
      "查看文档获取更多信息"
    ]
}
```

### 错误码设计思考

传统方案中使用数字错误码（如1001表示数据库错误）存在明显缺点：缺乏语义性，需查阅文档理解，定义者也易忘记含义。

因此，我们采用字符串作为错误码，优势明显：

1. 自解释性强
   - `ErrBadRequest`比`1001`直观明了
   - 错误码即文档
   - 便于代码审查和调试

2. 扩展性好
   - 可用模块前缀区分
   - 避免错误码冲突
   - 快速定位问题源

在和前端的对接过程中，某些接口出现错误了，前端会一头雾水，找服务端排查，有些可能只是参数问题，如果在响应的错误中有帮助解决的方案呢?能否简化对接复杂度?

为避免用户看到技术性错误，或者开发者缺乏上下文信息。我们将错误信息分为四个属性：

```go
type Error struct {
	Reason     string
	Msg        string
	Details    []string
	HTTPStatus int
}
```

每个字段的作用：

1. `reason` 字段
   - 使用大驼峰英语描述错误原因
   - 用于程序内部判断错误类型
   - 支持错误码映射到 HTTP 状态码

2. `msg` 字段
   - 使用开发者母语描述错误
   - 面向用户，提供友好的提示

3. `details` 字段
   - 提供错误扩展信息
   - 面向开发者，帮助调试
   - 可在生产环境调用 `web.SetRelease()` 隐藏，避免泄露敏感信息

4. `HTTPStatus`
   -  http 响应状态码
   -  默认为 400
   -  常用状态码 200,400,401 基本就够了，保持简单，少即是多

## 使用文档

1. 使用预定义的错误类型
   - `reason.ErrBadRequest`: 请求参数错误
   - `reason.ErrStore`: 数据库错误
   - `reason.ErrServer`: 服务器错误

2. 错误信息处理
   - 使用 `SetMsg()` 方法修改用户友好提示
   - 使用 `Withf()` 方法添加开发者帮助，增加错误上下文

采用一个 reason 原则，即 reason 相同则是同一个错误。

```go
   // e1 和 e2 是不是同一个错误!
   e1 = NewError("e1", "e1")
   e2 = NewError("e2", "e1")
```

```go
// e3 与 e2 是相同的错误
e2 := NewError("e2", "e2").SetHTTPStatus(200).With("e2-1")
e3 := fmt.Errorf("e3:%w", e2)
if !errors.Is(e3, e2) {
   t.Fatal("expect e3 is e2, but not")
}
```

```go
// 将错误转换为 *reason.Error 结构体
   var e5 *reason.Error
	if !errors.As(e4, &e5) {
		t.Fatal("expect e4 as e5, but not")
	}
```

## 总结

通过合理的分层和封装，我们实现了：

1. 统一的错误处理流程
2. 友好的用户提示
3. 详细的开发者信息
4. 安全的错误暴露

这种设计既保证了开发效率，又提升了用户体验。如果你正在寻找一个优雅的错误处理解决方案，不妨试试这个方案。

## 关于 goddd

本文介绍的错误处理是 [goddd](https://github.com/ixugo/goddd) 项目中的一个核心组件。goddd 是一个基于 DDD（领域驱动设计）理念的 Go 项目目标，它提供了一系列工具和最佳实践，帮助开发者构建可维护、可扩展的应用程序。

如果你对本文介绍的内容感兴趣，欢迎访问 [goddd 项目](https://github.com/ixugo/goddd) 了解更多细节。项目提供了完整的示例代码和详细的文档，可以帮助你快速上手。
