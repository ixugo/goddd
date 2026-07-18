package reason

import (
	"errors"
	"fmt"
	"strings"
)

// codes 记录所有已注册的 reason，用于启动期检测重复定义。
var codes = make(map[string]string, 8)

// CustomError 是 goddd 统一错误接口。
// 所有方法均返回新对象，不修改原错误（不可变语义）。
type CustomError interface {
	error
	ErrorInfoer

	// With 追加开发者排查信息到 details。
	With(args ...string) CustomError
	// Withf 格式化追加 details。
	Withf(format string, args ...any) CustomError
	// WithCause 包裹底层错误，保留 errors.Is/As 链路。
	WithCause(err error) CustomError
	// WithMsg 覆盖面向用户的提示信息。
	WithMsg(s string) CustomError
	// WithHTTPStatus 覆盖 HTTP 响应状态码。
	WithHTTPStatus(status int) CustomError

	// Deprecated: 使用 WithMsg 代替。
	SetMsg(s string) CustomError
	// Deprecated: 使用 WithHTTPStatus 代替。
	SetHTTPStatus(status int) CustomError
}

// ErrorInfoer 提供错误结构化信息的只读访问，
// 供 web.Fail 等响应层提取 reason/msg/details/HTTPStatus。
type ErrorInfoer interface {
	GetReason() string
	GetHTTPCode() int
	GetMessage() string
	GetDetails() []string
}

var _ CustomError = &Error{}

// Error 是 goddd 的统一错误结构体。
// Reason 和 Msg 面向 API 消费者，Details 面向开发者排查，
// HTTPStatus 映射 HTTP 状态码，Cause 保留底层错误链路。
type Error struct {
	Reason     string   `json:"reason"`
	Msg        string   `json:"msg"`
	Details    []string `json:"details"`
	HTTPStatus int      `json:"-"`
	Cause      error    `json:"-"`
}

// WithHTTPStatus 返回一个使用新 HTTP 状态码的错误副本，原错误不变。
func (e *Error) WithHTTPStatus(status int) CustomError {
	newErr := *e
	newErr.HTTPStatus = status
	return &newErr
}

// SetHTTPStatus 是 WithHTTPStatus 的旧名称，保留以兼容已有代码。
//
// Deprecated: 使用 WithHTTPStatus 代替。
func (e *Error) SetHTTPStatus(status int) CustomError {
	return e.WithHTTPStatus(status)
}

// WithMsg 返回一个使用新用户提示信息的错误副本，原错误不变。
func (e *Error) WithMsg(s string) CustomError {
	newErr := *e
	newErr.Msg = s
	return &newErr
}

// SetMsg 是 WithMsg 的旧名称，保留以兼容已有代码。
//
// Deprecated: 使用 WithMsg 代替。
func (e *Error) SetMsg(s string) CustomError {
	return e.WithMsg(s)
}

// Is 按 Reason 字符串比较错误，而非指针比较。
// 即使经过 With/WithMsg 产生了新对象，只要 Reason 相同就视为同一类错误。
func (e *Error) Is(err error) bool {
	if x, ok := err.(interface{ GetReason() string }); ok {
		return x.GetReason() == e.Reason
	}
	return false
}

// With 追加开发者排查信息到 details，返回新错误副本。
func (e *Error) With(args ...string) CustomError {
	newErr := *e
	newErr.Details = append(append(newErr.Details, e.Details...), args...)
	return &newErr
}

// Withf 格式化追加 details，返回新错误副本。
func (e *Error) Withf(format string, args ...any) CustomError {
	newErr := *e
	newErr.Details = append(append(newErr.Details, e.Details...), fmt.Sprintf(format, args...))
	return &newErr
}

// WithCause 返回一个携带底层错误的新错误，用于 errors.Is/As 链路解包。
// 首次调用直接设置 Cause；再次调用时通过 errors.Join 并列累加而非覆盖，
// 链式调用不丢前因。原错误不会被修改，符合 CustomError 不可变语义。
func (e *Error) WithCause(err error) CustomError {
	newErr := *e
	if newErr.Cause != nil {
		newErr.Cause = errors.Join(newErr.Cause, err)
	} else {
		newErr.Cause = err
	}
	return &newErr
}

// Unwrap 返回当前错误包裹的底层错误，供 errors.Is/errors.As 使用。
// 没有底层错误时返回 nil。
func (e *Error) Unwrap() error {
	return e.Cause
}

// Error 拼接 Msg 和 Details 返回完整的错误文本。
func (e *Error) Error() string {
	var msg strings.Builder
	msg.WriteString(e.Msg)
	for _, v := range e.Details {
		msg.WriteByte(';')
		msg.WriteString(v)
	}
	return msg.String()
}

// GetDetails 返回错误的详情列表。
func (e *Error) GetDetails() []string {
	return e.Details
}

// GetHTTPCode 返回错误对应的 HTTP 状态码。
func (e *Error) GetHTTPCode() int {
	return e.HTTPStatus
}

// GetMessage 返回面向用户的提示信息。
func (e *Error) GetMessage() string {
	return e.Msg
}

// GetReason 返回错误的机器可读标识。
func (e *Error) GetReason() string {
	return e.Reason
}

// NewError 创建一个自定义错误。
// 该函数要求每个 reason 全局唯一，若发现重复定义会立即 panic，
// 目的是在程序启动阶段就暴露冲突，避免不同模块使用相同的 reason
// 导致错误判断语义混乱。
func NewError(reason, msg string) CustomError {
	if _, ok := codes[reason]; ok {
		panic(fmt.Sprintf("err reason %s exists", reason))
	}
	codes[reason] = msg
	return &Error{Reason: reason, Msg: msg, HTTPStatus: 400}
}

// As 支持 errors.As 将错误提取为 *Error 类型。
// 匹配成功时将自身赋值给 target，符合标准库契约。
func (e *Error) As(target any) bool {
	if p, ok := target.(**Error); ok {
		*p = e
		return true
	}
	return false
}

// IsCustomError 判断 err 是否实现了 CustomError 接口。
// 建议直接使用 err.(CustomError) 类型断言。
func IsCustomError(err error) bool {
	_, ok := err.(CustomError)
	return ok
}
