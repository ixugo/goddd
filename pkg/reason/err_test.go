package reason

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewError_UniqueReason(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("重复 reason 应该 panic")
		}
	}()
	NewError("ErrBadRequest", "重复定义")
}

func TestWith(t *testing.T) {
	e := NewError("with_e1", "with e1")
	e2 := e.With("detail-a", "detail-b")

	if got := e2.GetDetails(); len(got) != 2 || got[0] != "detail-a" || got[1] != "detail-b" {
		t.Fatalf("With 应追加 details，got %v", got)
	}
	if len(e.GetDetails()) > 0 {
		t.Fatal("With 不应修改原错误的 details")
	}
}

func TestWithf(t *testing.T) {
	e := NewError("withf_e1", "withf e1")
	e2 := e.Withf("ID=%d 不合法", 42)

	details := e2.GetDetails()
	if len(details) != 1 || details[0] != "ID=42 不合法" {
		t.Fatalf("Withf 应格式化追加 details，got %v", details)
	}
	if len(e.GetDetails()) > 0 {
		t.Fatal("Withf 不应修改原错误的 details")
	}
}

func TestWithMsg(t *testing.T) {
	e := NewError("withmsg_e1", "原始消息")
	e2 := e.WithMsg("新消息")

	if e2.GetMessage() != "新消息" {
		t.Fatalf("WithMsg 应覆盖 msg，got %s", e2.GetMessage())
	}
	if e.GetMessage() != "原始消息" {
		t.Fatal("WithMsg 不应修改原错误的 msg")
	}
	if !errors.Is(e2, e) {
		t.Fatal("WithMsg 产生的新错误应与原错误 Is 匹配")
	}
}

func TestSetMsg_Deprecated(t *testing.T) {
	e := NewError("setmsg_e1", "e1")
	e2 := e.SetMsg("e2")
	if e2.GetMessage() != "e2" {
		t.Fatal("SetMsg 应与 WithMsg 行为一致")
	}
	if e2 == e {
		t.Fatal("SetMsg 应返回新对象")
	}
	if !errors.Is(e2, e) {
		t.Fatal("SetMsg 产生的新错误应与原错误 Is 匹配")
	}
}

func TestWithHTTPStatus(t *testing.T) {
	e := NewError("withhttpstatus_e1", "e1")
	if e.GetHTTPCode() != 400 {
		t.Fatalf("默认 HTTPStatus 应为 400，got %d", e.GetHTTPCode())
	}

	e2 := e.WithHTTPStatus(401)
	if e2.GetHTTPCode() != 401 {
		t.Fatalf("WithHTTPStatus 应覆盖状态码，got %d", e2.GetHTTPCode())
	}
	if e.GetHTTPCode() != 400 {
		t.Fatal("WithHTTPStatus 不应修改原错误的状态码")
	}
}

func TestSetHTTPStatus_Deprecated(t *testing.T) {
	e := NewError("sethttpstatus_e1", "e1")
	e2 := e.SetHTTPStatus(500)
	if e2.GetHTTPCode() != 500 {
		t.Fatalf("SetHTTPStatus 应与 WithHTTPStatus 行为一致，got %d", e2.GetHTTPCode())
	}
}

func TestWithCause(t *testing.T) {
	cause := errors.New("root cause")
	e := NewError("withcause_e1", "e1")
	e2 := e.WithCause(cause)

	if e.(*Error).Cause != nil {
		t.Fatal("WithCause 不应修改原错误")
	}
	if got := errors.Unwrap(e2); got != cause {
		t.Fatalf("Unwrap 应返回底层 cause，got %v", got)
	}
	if !errors.Is(e2, cause) {
		t.Fatal("errors.Is 应沿 Cause 链路匹配底层错误")
	}
}

func TestUnwrap_NoCause(t *testing.T) {
	e := NewError("unwrap_nil_e1", "e1")
	if got := errors.Unwrap(e); got != nil {
		t.Fatalf("无 Cause 时 Unwrap 应返回 nil，got %v", got)
	}
}

func TestWithCause_MultiLevel(t *testing.T) {
	cause := errors.New("root cause")
	e1 := NewError("multilevel_e1", "e1")
	e2 := e1.WithCause(cause)
	e3 := NewError("multilevel_e3", "e3").WithCause(e2)

	if !errors.Is(e3, e1) {
		t.Fatal("多层嵌套应通过 errors.Is 识别中间层")
	}
	if !errors.Is(e3, cause) {
		t.Fatal("多层嵌套应通过 errors.Is 识别根因")
	}
}

func TestIs_ByReason(t *testing.T) {
	e := NewError("is_reason_e1", "e1")
	e2 := e.With("detail").WithMsg("不同的 msg").WithHTTPStatus(500)

	if !errors.Is(e2, e) {
		t.Fatal("只要 Reason 相同，errors.Is 就应返回 true")
	}
}

func TestIs_DifferentReason(t *testing.T) {
	e1 := NewError("is_diff_e1", "e1")
	e2 := NewError("is_diff_e2", "e2")

	if errors.Is(e1, e2) {
		t.Fatal("不同 Reason 的错误不应匹配")
	}
}

func TestIs_ThroughFmtErrorf(t *testing.T) {
	e := NewError("is_wrap_e1", "e1")
	e2 := e.With("detail")
	e3 := fmt.Errorf("outer: %w", e2)

	if !errors.Is(e3, e) {
		t.Fatal("fmt.Errorf 包装后应通过 errors.Is 匹配")
	}
}

func TestAs_Direct(t *testing.T) {
	e := NewError("as_direct_e1", "e1").With("detail")
	var target *Error
	if !errors.As(e, &target) {
		t.Fatal("errors.As 应匹配 *Error")
	}
	if target.GetReason() != "as_direct_e1" {
		t.Fatalf("reason 应为 as_direct_e1，got %s", target.GetReason())
	}
}

func TestAs_ThroughFmtErrorf(t *testing.T) {
	e := NewError("as_wrap_e1", "e1").With("detail")
	wrapped := fmt.Errorf("outer: %w", e)

	var target *Error
	if !errors.As(wrapped, &target) {
		t.Fatal("fmt.Errorf 包装后应通过 errors.As 匹配")
	}
	if target.GetReason() != "as_wrap_e1" {
		t.Fatalf("reason 应为 as_wrap_e1，got %s", target.GetReason())
	}
}

func TestAs_Interface(t *testing.T) {
	e := NewError("as_iface_e1", "e1")
	wrapped := fmt.Errorf("outer: %w", e)

	var target CustomError
	if !errors.As(wrapped, &target) {
		t.Fatal("应通过 errors.As 匹配 CustomError 接口")
	}
	if target.GetReason() != "as_iface_e1" {
		t.Fatalf("reason 应为 as_iface_e1，got %s", target.GetReason())
	}
}

func TestAs_MethodContract(t *testing.T) {
	e := NewError("as_contract_e1", "e1").With("detail")
	re := e.(*Error)

	var target *Error
	ok := re.As(&target)
	if !ok {
		t.Fatal("As 方法应对 **Error 类型返回 true")
	}
	if target == nil {
		t.Fatal("As 方法应将自身赋值给 target")
	}
	if target.GetReason() != "as_contract_e1" {
		t.Fatalf("reason 应为 as_contract_e1，got %s", target.GetReason())
	}
}

func TestAs_WrongTarget(t *testing.T) {
	e := NewError("as_wrong_e1", "e1")
	re := e.(*Error)

	var target string
	ok := re.As(&target)
	if ok {
		t.Fatal("As 方法对非 **Error 类型应返回 false")
	}
}

func TestError_String(t *testing.T) {
	e := NewError("errstr_e1", "消息")
	if e.Error() != "消息" {
		t.Fatalf("Error() 应返回 msg，got %s", e.Error())
	}

	e2 := e.With("detail-a", "detail-b")
	if e2.Error() != "消息;detail-a;detail-b" {
		t.Fatalf("Error() 应拼接 msg 和 details，got %s", e2.Error())
	}
}

func TestIsCustomError(t *testing.T) {
	e := NewError("iscustom_e1", "e1")
	if !IsCustomError(e) {
		t.Fatal("NewError 返回值应是 CustomError")
	}

	stdErr := errors.New("standard error")
	if IsCustomError(stdErr) {
		t.Fatal("标准 error 不应匹配 CustomError")
	}
}

func TestChainCombination(t *testing.T) {
	cause := errors.New("db connection refused")
	e := NewError("chain_e1", "数据错误").
		WithCause(cause).
		With("查询用户失败").
		WithMsg("服务暂时不可用").
		WithHTTPStatus(503)

	if e.GetReason() != "chain_e1" {
		t.Fatalf("reason 应为 chain_e1，got %s", e.GetReason())
	}
	if e.GetMessage() != "服务暂时不可用" {
		t.Fatalf("msg 应为 '服务暂时不可用'，got %s", e.GetMessage())
	}
	if e.GetHTTPCode() != 503 {
		t.Fatalf("HTTPStatus 应为 503，got %d", e.GetHTTPCode())
	}
	if !errors.Is(e, cause) {
		t.Fatal("链式调用后应能通过 errors.Is 匹配底层 cause")
	}
}
