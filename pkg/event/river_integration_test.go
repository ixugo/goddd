package event_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/ixugo/goddd/pkg/event"
)

// 模拟 River 的 Job 参数结构（实际使用时为 river.JobArgs 实现）
type cleanupUserHomesArgs struct {
	UserID int `json:"user_id"`
}

// mockRiverClient 模拟 River 客户端行为：入队 = 追加到内存列表
type mockRiverClient struct {
	mu   sync.Mutex
	jobs [][]byte
}

func (m *mockRiverClient) Insert(_ context.Context, args any) error {
	data, _ := json.Marshal(args)
	m.mu.Lock()
	m.jobs = append(m.jobs, data)
	m.mu.Unlock()
	return nil
}

// TestBus_RiverIntegration 演示 event.Bus 与 River 的集成模式：
// Bus handler 内部调用 River 入队，实现同步通知 + 持久化异步处理。
func TestBus_RiverIntegration(t *testing.T) {
	river := &mockRiverClient{}
	bus := event.NewBus[testEvent]()

	// 注册 River handler：将事件转为 River Job 入队
	bus.Register("river:cleanup_homes", func(ctx context.Context, e testEvent) error {
		return river.Insert(ctx, cleanupUserHomesArgs{UserID: e.UserID})
	})

	// 同时注册一个同步 handler（日志记录等）
	var logged bool
	bus.Register("sync:log", func(_ context.Context, _ testEvent) error {
		logged = true
		return nil
	})

	// 触发事件
	if err := bus.Notify(context.Background(), testEvent{UserID: 42}); err != nil {
		t.Fatalf("Notify err: %v", err)
	}

	// 验证 River 收到了 job
	if len(river.jobs) != 1 {
		t.Fatalf("expected 1 river job, got %d", len(river.jobs))
	}
	var args cleanupUserHomesArgs
	if err := json.Unmarshal(river.jobs[0], &args); err != nil {
		t.Fatalf("unmarshal job: %v", err)
	}
	if args.UserID != 42 {
		t.Fatalf("expected UserID=42, got %d", args.UserID)
	}

	// 验证同步 handler 也被调用
	if !logged {
		t.Fatal("sync handler should also be called")
	}
}
