package reason

import (
	"fmt"
	"strings"
)

var codes = make(map[string]string, 8)

type CustomError interface {
	error
	ErrorInfoer
	With(args ...string) CustomError
	Withf(format string, args ...any) CustomError
	SetMsg(s string) CustomError
	SetHTTPStatus(status int) CustomError
}

type ErrorInfoer interface {
	GetReason() string
	GetHTTPCode() int
	GetMessage() string
	GetDetails() []string
}

var _ CustomError = &Error{}

type Error struct {
	Reason     string   `json:"reason"`
	Msg        string   `json:"msg"`
	Details    []string `json:"details"`
	HTTPStatus int      `json:"-"`
}

// SetHTTPStatus implements CustomError.
func (e *Error) SetHTTPStatus(status int) CustomError {
	newErr := *e
	newErr.HTTPStatus = status
	return &newErr
}

// SetMsg implements CustomError.
func (e *Error) SetMsg(s string) CustomError {
	newErr := *e
	newErr.Msg = s
	return &newErr
}

func (e *Error) Is(err error) bool {
	if x, ok := err.(interface{ GetReason() string }); ok {
		return x.GetReason() == e.Reason
	}
	return false
}

// With implements CustomError.
func (e *Error) With(args ...string) CustomError {
	newErr := *e
	newErr.Details = append(append(newErr.Details, e.Details...), args...)
	return &newErr
}

// Withf implements CustomError.
func (e *Error) Withf(format string, args ...any) CustomError {
	newErr := *e
	newErr.Details = append(append(newErr.Details, e.Details...), fmt.Sprintf(format, args...))
	return &newErr
}

// Error implements CustomError.
func (e *Error) Error() string {
	var msg strings.Builder
	msg.WriteString(e.Msg)
	for _, v := range e.Details {
		msg.WriteString(";" + v)
	}
	return msg.String()
}

// GetDetails implements CustomError.
func (e *Error) GetDetails() []string {
	return e.Details
}

// GetHTTPCode implements CustomError.
func (e *Error) GetHTTPCode() int {
	return e.HTTPStatus
}

// GetMessage implements CustomError.
func (e *Error) GetMessage() string {
	return e.Msg
}

// GetReason implements CustomError.
func (e *Error) GetReason() string {
	return e.Reason
}

// NewError ..
func NewError(reason, msg string) CustomError {
	if _, ok := codes[reason]; ok {
		panic(fmt.Sprintf("err reason %s exists", reason))
	}
	codes[reason] = msg
	return &Error{Reason: reason, Msg: msg, HTTPStatus: 400}
}

func (e *Error) As(target any) bool {
	_, ok := target.(*Error)
	return ok
}

// IsCustomError 是否自定义的错误
// 需要断言后的类型，建议直接使用 `err.(CustomError)` 语法
func IsCustomError(err error) bool {
	_, ok := err.(CustomError)
	return ok
}
