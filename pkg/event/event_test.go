package event_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ixugo/goddd/pkg/event"
)

type testEvent struct {
	UserID int
}

func TestBus_Notify(t *testing.T) {
	bus := event.NewBus[testEvent]()

	called := make(map[string]bool)
	bus.Register("home", func(_ context.Context, e testEvent) error {
		called["home"] = true
		return nil
	})
	bus.Register("product", func(_ context.Context, e testEvent) error {
		called["product"] = true
		return nil
	})

	if err := bus.Notify(context.Background(), testEvent{UserID: 7}); err != nil {
		t.Fatalf("Notify err: %v", err)
	}
	if !called["home"] || !called["product"] {
		t.Fatalf("expected both called, got: %v", called)
	}
}

func TestBus_Notify_StopsOnError(t *testing.T) {
	bus := event.NewBus[testEvent]()
	errBoom := errors.New("boom")

	bus.Register("fail", func(_ context.Context, _ testEvent) error { return errBoom })

	err := bus.Notify(context.Background(), testEvent{UserID: 1})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got: %v", err)
	}
}

func TestBus_EmptyHandlers(t *testing.T) {
	bus := event.NewBus[testEvent]()
	if err := bus.Notify(context.Background(), testEvent{UserID: 1}); err != nil {
		t.Fatalf("empty bus should not error: %v", err)
	}
}

func TestBus_Unregister(t *testing.T) {
	bus := event.NewBus[testEvent]()

	called := false
	bus.Register("temp", func(_ context.Context, _ testEvent) error {
		called = true
		return nil
	})
	bus.Unregister("temp")

	if err := bus.Notify(context.Background(), testEvent{UserID: 1}); err != nil {
		t.Fatalf("Notify err: %v", err)
	}
	if called {
		t.Fatal("unregistered handler should not be called")
	}
}

func TestBus_Register_DuplicateKey_Panics(t *testing.T) {
	bus := event.NewBus[testEvent]()
	bus.Register("dup", func(_ context.Context, _ testEvent) error { return nil })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate key")
		}
	}()
	bus.Register("dup", func(_ context.Context, _ testEvent) error { return nil })
}

// 编译期断言：Bus 实现 Notifier 接口
var _ event.Notifier[testEvent] = (*event.Bus[testEvent])(nil)
