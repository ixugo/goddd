// Package assert 提供最基础的测试断言函数，替代 testify/assert 的常用功能。
package assert

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// failf 统一上报断言失败，msgAndArgs 的第一个参数是 format 字符串，后面是参数。
// 返回 false 表示断言未通过，作为各断言函数的返回值。
func failf(t testing.TB, baseMsg string, msgAndArgs ...any) bool {
	t.Helper()
	if len(msgAndArgs) > 0 {
		baseMsg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + baseMsg
	}
	t.Error(baseMsg)
	return false
}

// isNil 判断值是否为 nil，兼容指针、切片、map 等携带类型信息的 nil。
func isNil(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// isEmpty 判断值是否为空：nil、长度为零的字符串/切片/map/通道，或对应类型的零值。
func isEmpty(value any) bool {
	if isNil(value) {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		return rv.Len() == 0
	default:
		return reflect.DeepEqual(value, reflect.Zero(rv.Type()).Interface())
	}
}

// Equal 断言两个值相等，不等时报告错误并标记测试失败。
// msgAndArgs 支持可选的格式化消息，第一个参数是 format 字符串，后面是参数。
// 返回是否断言通过，便于用 if !assert.Equal(...) { return } 实现立即终止。
func Equal(t testing.TB, expected, actual any, msgAndArgs ...any) bool {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		return failf(t, fmt.Sprintf("期望 %v (类型 %T)，实际 %v (类型 %T)", expected, expected, actual, actual), msgAndArgs...)
	}
	return true
}

// True 断言值为 true，返回是否断言通过。
func True(t testing.TB, value bool, msgAndArgs ...any) bool {
	t.Helper()
	if !value {
		return failf(t, "期望 true，实际 false", msgAndArgs...)
	}
	return true
}

// False 断言值为 false，返回是否断言通过。
func False(t testing.TB, value bool, msgAndArgs ...any) bool {
	t.Helper()
	if value {
		return failf(t, "期望 false，实际 true", msgAndArgs...)
	}
	return true
}

// NoError 断言 err 为 nil，返回是否断言通过。
func NoError(t testing.TB, err error, msgAndArgs ...any) bool {
	t.Helper()
	if err != nil {
		return failf(t, fmt.Sprintf("期望无错误，实际: %v", err), msgAndArgs...)
	}
	return true
}

// Contains 断言 s 包含 substr。支持 string 和 []byte 类型，返回是否断言通过。
func Contains(t testing.TB, s any, substr string, msgAndArgs ...any) bool {
	t.Helper()
	var content string
	switch v := s.(type) {
	case string:
		content = v
	case []byte:
		content = string(v)
	default:
		return failf(t, fmt.Sprintf("Contains 不支持类型 %T，仅支持 string 和 []byte", s), msgAndArgs...)
	}
	if !strings.Contains(content, substr) {
		return failf(t, fmt.Sprintf("期望包含 %q，实际 %q", substr, content), msgAndArgs...)
	}
	return true
}

// Error 断言 err 不为 nil，返回是否断言通过。
func Error(t testing.TB, err error, msgAndArgs ...any) bool {
	t.Helper()
	if err == nil {
		return failf(t, "期望有错误，实际为 nil", msgAndArgs...)
	}
	return true
}

// ErrorIs 断言 err 链上包含 target 错误，返回是否断言通过。
func ErrorIs(t testing.TB, err, target error, msgAndArgs ...any) bool {
	t.Helper()
	if !errors.Is(err, target) {
		return failf(t, fmt.Sprintf("期望错误链包含 %v，实际: %v", target, err), msgAndArgs...)
	}
	return true
}

// Nil 断言值为 nil，兼容携带类型信息的 nil（如 (*int)(nil)），返回是否断言通过。
func Nil(t testing.TB, value any, msgAndArgs ...any) bool {
	t.Helper()
	if !isNil(value) {
		return failf(t, fmt.Sprintf("期望 nil，实际 %v (类型 %T)", value, value), msgAndArgs...)
	}
	return true
}

// NotNil 断言值不为 nil，兼容携带类型信息的 nil，返回是否断言通过。
func NotNil(t testing.TB, value any, msgAndArgs ...any) bool {
	t.Helper()
	if isNil(value) {
		return failf(t, "期望非 nil，实际为 nil", msgAndArgs...)
	}
	return true
}

// Empty 断言值为空：nil、零长字符串/切片/map/通道，或类型零值，返回是否断言通过。
func Empty(t testing.TB, value any, msgAndArgs ...any) bool {
	t.Helper()
	if !isEmpty(value) {
		return failf(t, fmt.Sprintf("期望为空，实际 %v (类型 %T)", value, value), msgAndArgs...)
	}
	return true
}

// NotEmpty 断言值非空：非 nil、长度大于零，且不是类型零值，返回是否断言通过。
func NotEmpty(t testing.TB, value any, msgAndArgs ...any) bool {
	t.Helper()
	if isEmpty(value) {
		return failf(t, "期望非空，实际为空", msgAndArgs...)
	}
	return true
}

// Len 断言值的长度等于 length，仅支持字符串/数组/切片/map/通道，返回是否断言通过。
func Len(t testing.TB, value any, length int, msgAndArgs ...any) bool {
	t.Helper()
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		if rv.Len() != length {
			return failf(t, fmt.Sprintf("期望长度 %d，实际 %d", length, rv.Len()), msgAndArgs...)
		}
		return true
	default:
		return failf(t, fmt.Sprintf("Len 不支持类型 %T", value), msgAndArgs...)
	}
}

// Zero 断言值是其类型的零值，返回是否断言通过。
func Zero(t testing.TB, value any, msgAndArgs ...any) bool {
	t.Helper()
	if !isEmpty(value) {
		return failf(t, fmt.Sprintf("期望零值，实际 %v (类型 %T)", value, value), msgAndArgs...)
	}
	return true
}

// Eventually 断言 condition 在 waitFor 时间内最终返回 true，每隔 tick 轮询一次。
// 用于替代 sleep 等待异步结果，避免测试因固定等待时间而 flaky，返回是否断言通过。
func Eventually(t testing.TB, condition func() bool, waitFor, tick time.Duration, msgAndArgs ...any) bool {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(tick)
	}
	return failf(t, fmt.Sprintf("条件在 %v 内未达成", waitFor), msgAndArgs...)
}

// Never 断言 condition 在 waitFor 时间内始终返回 false，每隔 tick 检查一次。
// 一旦条件成立立即报错，用于验证某事在观察窗口内绝不发生，返回是否断言通过。
func Never(t testing.TB, condition func() bool, waitFor, tick time.Duration, msgAndArgs ...any) bool {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		if condition() {
			return failf(t, "条件在观察窗口内不应成立", msgAndArgs...)
		}
		time.Sleep(tick)
	}
	return true
}
