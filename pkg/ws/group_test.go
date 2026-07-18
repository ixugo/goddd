package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

// groupTestClient 拨号并完成鉴权，返回连接
func groupTestClient(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return nil
	}
	if !assert.NoError(t, conn.WriteJSON(map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": token},
	})) {
		return nil
	}
	var resp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return nil
	}
	assert.Equal(t, MsgTypeAuthOK, resp["type"])
	return conn
}

// findClientByID 按业务 ID 在已认证客户端列表中定位连接
func findClientByID(t *testing.T, hub *Hub, id string) *Client {
	t.Helper()
	for _, c := range hub.GetClients() {
		if c.ID() == id {
			return c
		}
	}
	t.Fatalf("客户端 %s 不存在", id)
	return nil
}

// TestGroupSendToMembers 组内投递：仅组成员收到消息，组外连接不受影响
func TestGroupSendToMembers(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, _ := data["token"].(string)
		return "user_" + token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	connA := groupTestClient(t, wsURL, "a")
	defer connA.Close()
	connB := groupTestClient(t, wsURL, "b")
	defer connB.Close()
	connC := groupTestClient(t, wsURL, "c")
	defer connC.Close()

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 3
	}, 2*time.Second, 20*time.Millisecond)

	findClientByID(t, hub, "user_a").JoinGroup("room1")
	findClientByID(t, hub, "user_b").JoinGroup("room1")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 2
	}, 2*time.Second, 20*time.Millisecond)

	err := hub.SendToGroup(context.Background(), "room1", NewMessage("room_msg", nil))
	if !assert.NoError(t, err) {
		return
	}

	for _, conn := range []*websocket.Conn{connA, connB} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg map[string]any
		if !assert.NoError(t, conn.ReadJSON(&msg)) {
			return
		}
		assert.Equal(t, "room_msg", msg["type"])
	}

	// 组外连接不应收到
	connC.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var msg map[string]any
	assert.Error(t, connC.ReadJSON(&msg))
}

// TestGroupLeaveIdempotent 退组幂等：重复退组不 panic，退组后不再收到组消息
func TestGroupLeaveIdempotent(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_solo", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn := groupTestClient(t, wsURL, "x")
	defer conn.Close()

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_solo")
	client.JoinGroup("room1")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 1
	}, 2*time.Second, 20*time.Millisecond)

	client.LeaveGroup("room1")
	client.LeaveGroup("room1")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 0
	}, 2*time.Second, 20*time.Millisecond)

	if !assert.NoError(t, hub.SendToGroup(context.Background(), "room1", NewMessage("room_msg", nil))) {
		return
	}
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var msg map[string]any
	assert.Error(t, conn.ReadJSON(&msg))
}

// TestGroupDisconnectCleanup 断开连接自动清出所有分组，无需业务手动清理
func TestGroupDisconnectCleanup(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_tmp", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn := groupTestClient(t, wsURL, "x")

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_tmp")
	client.JoinGroup("room1")
	client.JoinGroup("room2")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 1 && hub.GroupSize("room2") == 1
	}, 2*time.Second, 20*time.Millisecond)

	if !assert.NoError(t, conn.Close()) {
		return
	}
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 0 && hub.GroupSize("room2") == 0
	}, 2*time.Second, 20*time.Millisecond, "断开后应自动清出所有分组")
}

// TestGroupSlowConsumerSkipped 组内慢连接跳过：发送队列积压时不阻塞投递，正常成员照收
func TestGroupSlowConsumerSkipped(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, _ := data["token"].(string)
		return "user_" + token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	slow := groupTestClient(t, wsURL, "slow")
	defer slow.Close()
	fast := groupTestClient(t, wsURL, "fast")
	defer fast.Close()

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 2
	}, 2*time.Second, 20*time.Millisecond)

	slowClient := findClientByID(t, hub, "user_slow")
	fastClient := findClientByID(t, hub, "user_fast")
	slowClient.JoinGroup("room1")
	fastClient.JoinGroup("room1")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 2
	}, 2*time.Second, 20*time.Millisecond)

	// 填满慢连接的发送队列，模拟消费积压
	for range hub.config.MessageQueueSize {
		slowClient.send <- NewMessage("flood", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !assert.NoError(t, hub.SendToGroup(ctx, "room1", NewMessage("room_msg", nil))) {
		return
	}

	fast.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	if !assert.NoError(t, fast.ReadJSON(&msg)) {
		return
	}
	assert.Equal(t, "room_msg", msg["type"])

	// 慢连接不被剔除、连接保持存活
	assert.NoError(t, slowClient.ctx.Err())
}

// TestGroupEdgeCases 边界：空组名忽略、不存在的组投递成功、异步投递可用
func TestGroupEdgeCases(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_edge", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn := groupTestClient(t, wsURL, "x")
	defer conn.Close()

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_edge")

	// 空组名为空操作
	client.JoinGroup("")
	client.LeaveGroup("")
	assert.Equal(t, 0, hub.GroupSize(""))

	// 向不存在的分组投递视为成功
	if !assert.NoError(t, hub.SendToGroup(context.Background(), "no_such_room", NewMessage("m", nil))) {
		return
	}

	// 异步投递
	client.JoinGroup("room_async")
	assert.Eventually(t, func() bool {
		return hub.GroupSize("room_async") == 1
	}, 2*time.Second, 20*time.Millisecond)
	if !assert.NoError(t, hub.SendToGroupAsync(context.Background(), "room_async", NewMessage("async_room", nil))) {
		return
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	if !assert.NoError(t, conn.ReadJSON(&msg)) {
		return
	}
	assert.Equal(t, "async_room", msg["type"])
}

// TestJoinSkipsDeadClient 连接在 join 事件排队期间已断开时不予登记，防止幽灵连接
func TestJoinSkipsDeadClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	dead := newClient(nil, hub, nil)
	dead.cancel() // 模拟 join 入队后连接即断开的重排窗口

	hub.join <- dead
	assert.Never(t, func() bool {
		return len(hub.GetClients()) > 0
	}, 300*time.Millisecond, 50*time.Millisecond, "已死连接不应登记")
}
