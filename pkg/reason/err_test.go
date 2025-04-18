package reason

import (
	"errors"
	"fmt"
	"testing"
)

func TestSetMsg(t *testing.T) {
	e := NewError("e1", "e1")
	e2 := e.SetMsg("e2")
	if e2.GetMessage() != "e2" {
		t.Fatal("expect e message is e2, but not")
	}
	// 从内存地址上说，是不同的错误
	if e2 == e {
		t.Fatal("expect e2 is not e, but is")
	}
	// 从类型上说，是同一个错误
	if !errors.Is(e2, e) {
		t.Fatal("expect e2 is e, but not")
	}

	// 添加一个错误信息
	e3 := e.With("test")
	if e3.GetDetails()[0] != "test" {
		t.Fatal("expect e3 message is e1;test, but not")
	}
	// e3 添加的错误信息，e 是没有的
	if len(e.GetDetails()) > 0 {
		t.Fatal("expect e details is empty, but not")
	}
}

// 错误包裹错误
func TestErrWrap(t *testing.T) {
	var e1 error
	e1 = NewError("e1", "e1")
	e2 := NewError("e2", "e2").SetHTTPStatus(200).With("e2-1")
	e3 := fmt.Errorf("e3:%w", e2)

	// e3 包裹了 e2，所以 e3 是 e2
	if !errors.Is(e3, e2) {
		t.Fatal("expect e3 is e2, but not")
	}

	// e2 没有包裹 e1，所以 e2 不是 e1
	if errors.Is(e2, e1) {
		t.Fatal("expect e3 is not e1, but is")
	}

	// 解包裹 e3，得到 e2
	if err := errors.Unwrap(e3); !errors.Is(err, e2) {
		t.Fatal("expect err is e2, but not")
	}

	// 包裹 e3，得到 e4，e4-e3-e2，所以 e4 是 e2
	e4 := fmt.Errorf("e4:%w", e3)
	if !errors.Is(e4, e2) {
		t.Fatal("expect e4 is e2, but not")
	}

	var e5 *Error
	if !errors.As(e4, &e5) {
		t.Fatal("expect e4 as e5, but not")
	}

	if e5.GetReason() != e2.GetReason() {
		t.Fatal("expect e5 reason is e2 reason, but not")
	}

	if e5.GetHTTPCode() != e2.GetHTTPCode() {
		t.Fatal("expect e5 http code is e2 http code, but not")
	}

	if e5.GetMessage() != e2.GetMessage() {
		t.Fatal("expect e5 message is e2 message, but not")
	}
}
