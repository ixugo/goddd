package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

// TestBroadcastSkipsSlowConsumer 广播跳过发送队列满的慢连接，但不剔除、不关闭
func TestBroadcastSkipsSlowConsumer(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.MessageQueueSize = 1
	})
	defer hub.Close()

	// 构造无网络假客户端：队列满且无人消费
	c := &Client{send: make(chan Message, 1)}
	c.isAuth.Store(true)
	c.id.Store("slow")
	c.ctx, c.cancel = context.WithCancel(context.Background())

	hub.join <- c
	hub.addToID <- c
	time.Sleep(50 * time.Millisecond)

	// 占满队列后广播，触发跳过
	c.send <- NewMessage("fill", nil)
	hub.Broadcast(NewMessage("b", nil))
	time.Sleep(100 * time.Millisecond)

	// 慢连接仍在册、未被关闭，删除只能由 leave 事件执行
	clients := hub.GetClients()
	assert.Len(t, clients, 1)
	assert.NoError(t, c.ctx.Err(), "广播不得关闭慢连接")
}

// TestSendToClientAsync 异步投递：入队成功即返回，消息正常送达
func TestSendToClientAsync(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_async", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}})) {
		return
	}
	var resp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}

	time.Sleep(100 * time.Millisecond)

	err = hub.SendToClientAsync(context.Background(), "user_async", NewMessage("async_msg", nil))
	if !assert.NoError(t, err) {
		return
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	assert.Equal(t, "async_msg", resp["type"])
}

// TestEmptyAuthIDKeepsUUID 鉴权回调返回空 ID 时保留连接建立时分配的 UUID
func TestEmptyAuthIDKeepsUUID(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	var resp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	assert.Eventually(t, func() bool {
		clients := hub.GetClients()
		if len(clients) != 1 {
			return false
		}
		_, err := uuid.Parse(clients[0].ID())
		return err == nil
	}, 2*time.Second, 50*time.Millisecond, "空 ID 应保留 UUID")
}

func TestHubLifecycleCallbacks(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	connected := make(chan struct{}, 1)
	disconnected := make(chan struct{}, 1)
	errored := make(chan error, 1)
	defaultHit := make(chan string, 1)

	hub.SetAuthHandler(func(message Message) (string, error) { return "user_cb", nil })
	hub.SetConnectHandler(func(client *Client) error {
		connected <- struct{}{}
		return nil
	})
	hub.SetDisconnectHandler(func(client *Client, err error) {
		disconnected <- struct{}{}
	})
	hub.SetErrorHandler(func(client *Client, err error) {
		errored <- err
	})
	hub.SetDefaultHandler(HandlerFunc(func(client *Client, message Message) error {
		defaultHit <- message.Type()
		return nil
	}))

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("连接回调未触发")
	}

	// 鉴权后发送未注册类型，触发默认处理器
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	var resp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": "未注册"})) {
		return
	}
	select {
	case got := <-defaultHit:
		assert.Equal(t, "未注册", got)
	case <-time.After(2 * time.Second):
		t.Fatal("默认处理器未触发")
	}

	// 发送非法 JSON，触发错误处理器
	if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("这不是JSON"))) {
		return
	}
	select {
	case err := <-errored:
		assert.ErrorIs(t, err, ErrInvalidMessage)
	case <-time.After(2 * time.Second):
		t.Fatal("错误处理器未触发")
	}

	if !assert.NoError(t, conn.Close()) {
		return
	}
	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("断开回调未触发")
	}
}

// TestReadPumpErrorPaths 覆盖读泵的错误分支：未鉴权发业务消息、处理器报错回传

func TestClosedHubOperations(t *testing.T) {
	hub := NewHub()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	hub.Close()
	hub.Close() // 幂等

	msg := NewMessage("m", nil)
	hub.Broadcast(msg) // 关闭后广播为空操作
	assert.ErrorIs(t, hub.SendToClient(context.Background(), "x", msg), ErrHubClosed)
	assert.ErrorIs(t, hub.SendToClientAsync(context.Background(), "x", msg), ErrHubClosed)
	assert.ErrorIs(t, hub.SendToGroup(context.Background(), "g", msg), ErrHubClosed)
	assert.ErrorIs(t, hub.SendToGroupAsync(context.Background(), "g", msg), ErrHubClosed)
	assert.Zero(t, hub.GroupSize("g"))
	assert.Empty(t, hub.GetClients())

	// 关闭后升级请求返回 503
	resp, err := http.Get(server.URL)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// 关闭后登记鉴权客户端立即返回，不阻塞
	done := make(chan struct{})
	go func() {
		hub.addAuthenticatedClient(context.Background(), newSyntheticClient(1))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Hub 关闭后 addAuthenticatedClient 阻塞")
	}
}

// TestJoinClientOverLimit 连接数超限时新连接被拒绝并关闭，不登记进 Hub
func TestJoinClientOverLimit(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.MaxConnections = 0 // 任何连接都超限
	})
	defer hub.Close()

	c := newClient(nil, hub, nil)
	hub.join <- c
	assert.Eventually(t, func() bool {
		return c.ctx.Err() != nil
	}, 2*time.Second, 20*time.Millisecond, "超限连接必须被关闭")
	assert.Empty(t, hub.GetClients())
}

// TestSendToDeliveryFailures 定向投递的两类失败：队列满返回 ErrMessageQueueFull，连接已死返回 ErrConnectionClosed
func TestSendToDeliveryFailures(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.MessageQueueSize = 1
	})
	defer hub.Close()

	// 队列满客户端：容量 1 且无人消费
	full := newClient(nil, hub, nil)
	full.setID("full")
	full.setAuth(true)
	full.send <- NewMessage("占位", nil)

	// 已死客户端：队列同样占满，使 select 只剩 ctx.Done 可走
	dead := newClient(nil, hub, nil)
	dead.setID("dead")
	dead.setAuth(true)
	dead.send <- NewMessage("占位", nil)
	dead.cancel()

	hub.join <- full
	hub.join <- dead
	hub.addToID <- full
	hub.addToID <- dead
	time.Sleep(100 * time.Millisecond)

	err := hub.SendToClient(context.Background(), "full", NewMessage("m", nil))
	assert.ErrorIs(t, err, ErrMessageQueueFull)

	err = hub.SendToClient(context.Background(), "dead", NewMessage("m", nil))
	assert.ErrorIs(t, err, ErrConnectionClosed)

	// nil 消息广播走提前返回分支，不得 panic
	hub.Broadcast(nil)
	time.Sleep(50 * time.Millisecond)
}

// TestExampleChatServerWS 通过真实 WebSocket 验证示例服务器的鉴权与在线用户查询流程。
// 注意不能发 chat/private 消息：示例处理器对 metadata["username"] 做强类型断言，
// 而示例鉴权回调无法写 metadata，强断言会 panic，这两条路只能用直接调用测。

func TestSendWithCanceledContext(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := NewMessage("m", nil)
	var canceled int
	for range 200 {
		for _, err := range []error{
			hub.SendToClient(ctx, "nobody", msg),
			hub.SendToClientAsync(ctx, "nobody", msg),
			hub.SendToGroup(ctx, "g", msg),
			hub.SendToGroupAsync(ctx, "g", msg),
		} {
			if errors.Is(err, context.Canceled) {
				canceled++
			}
		}
	}
	assert.True(t, canceled > 0, "已取消 ctx 下应至少有一次返回 context.Canceled")
}

// TestServeHTTPUpgradeFailure 非 WebSocket 请求走升级失败分支，返回 400
func TestServeHTTPUpgradeFailure(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestRemoveFromIDMapMultiConn 同一业务 ID 的两个连接断开其一时，ID 映射保留另一个
func TestRemoveFromIDMapMultiConn(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	c1 := newClient(nil, hub, nil)
	c1.setID("same_user")
	c1.setAuth(true)
	c2 := newClient(nil, hub, nil)
	c2.setID("same_user")
	c2.setAuth(true)

	hub.join <- c1
	hub.join <- c2
	hub.addToID <- c1
	hub.addToID <- c2
	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 2
	}, 2*time.Second, 20*time.Millisecond)

	// c1 离开后，定向投递仍能命中 c2
	hub.leave <- c1
	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)
	if !assert.NoError(t, hub.SendToClient(context.Background(), "same_user", NewMessage("m", nil))) {
		return
	}
	assert.Equal(t, "m", drainSend(t, c2).Type())
}

// TestExampleRoleHandlers 直接调用管理员与普通用户处理器，覆盖各命令分支

func TestLifecycleCallbackPanic(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetConnectHandler(func(client *Client) error {
		panic("connect panic")
	})
	hub.SetDisconnectHandler(func(client *Client, err error) {
		panic("disconnect panic")
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}

	// 触发 connect 回调 panic 后断开，触发 disconnect 回调 panic
	_ = conn.Close()
	time.Sleep(200 * time.Millisecond)

	// Hub 未被 panic 击垮，新连接照常接入
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn2.Close()

	if !assert.NoError(t, conn2.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	var authResp map[string]any
	if !assert.NoError(t, conn2.ReadJSON(&authResp)) {
		return
	}
	assert.Equal(t, MsgTypeAuthOK, authResp["type"])
	assert.Len(t, hub.GetClients(), 1)
}
