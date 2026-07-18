package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

// TestConcurrentSendAndClose 并发 Send 与连接关闭不得产生 panic、数据竞争（配合 -race）
// 或非预期错误：Send 只允许成功或返回 ErrConnectionClosed
func TestConcurrentSendAndClose(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_race", nil
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

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 50*time.Millisecond)

	c := hub.GetClients()[0]
	var unexpected atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 100 {
				// 并发关闭期间，Send 只允许成功或返回连接已关闭，不得出现其他错误
				if err := c.Send(context.Background(), NewMessage("spam", j)); err != nil && !errors.Is(err, ErrConnectionClosed) {
					unexpected.Add(1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	_ = c.close()
	wg.Wait()
	assert.Zero(t, unexpected.Load(), "并发 Send 与 Close 期间出现了非预期错误")
}

func TestClientRequest(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_req", nil
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

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)
	assert.NotNil(t, hub.GetClients()[0].Request(), "真实升级而来的连接必须持有原始请求")

	assert.Nil(t, newClient(nil, hub, nil).Request(), "合成客户端无原始请求")
}

// TestHubLifecycleCallbacks 验证连接、断开、错误、默认处理器四类回调在真实连接上依次触发

func TestReadPumpErrorPaths(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) { return "user_rp", nil })
	hub.Handle("boom", HandlerFunc(func(client *Client, message Message) error {
		return errors.New("处理器爆炸了")
	}))

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 未鉴权直接发业务消息，应收到鉴权失败错误
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": "chat"})) {
		return
	}
	msg := readUntilType(t, conn, MsgTypeError)
	assert.Equal(t, ErrAuthFailed.Error(), msg["msg"])

	// 鉴权
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	readUntilType(t, conn, MsgTypeAuthOK)

	// 鉴权后调用会报错的处理器，错误内容应回传给客户端
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": "boom"})) {
		return
	}
	msg = readUntilType(t, conn, MsgTypeError)
	assert.Equal(t, "处理器爆炸了", msg["msg"])

	// 缺少 type 字段的消息视为无效消息
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"foo": "bar"})) {
		return
	}
	msg = readUntilType(t, conn, MsgTypeError)
	assert.Equal(t, ErrInvalidMessage.Error(), msg["msg"])
}

// TestWritePumpPing 心跳间隔缩到毫秒级，验证写泵定时 Ping 分支不会断开正常连接
func TestWritePumpPing(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.HeartbeatInterval = 50 * time.Millisecond
	})
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) { return "user_ping", nil })

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

	// 跨过多个心跳周期后连接仍然可用，说明 Ping 分支正常执行
	time.Sleep(200 * time.Millisecond)
	if !assert.NoError(t, hub.SendToClient(context.Background(), "user_ping", NewMessage("after_ping", nil))) {
		return
	}
	msg := readUntilType(t, conn, "after_ping")
	if !assert.NotNil(t, msg, "心跳周期后应能读到 after_ping 消息") {
		return
	}
	assert.Equal(t, "after_ping", msg["type"])
}

// TestClosedHubOperations 关闭后的所有公开操作必须安全返回既定值，不得 panic 或阻塞

type badMessage struct{}

func (badMessage) Type() string             { return "bad" }
func (badMessage) Data() json.RawMessage    { return nil }
func (badMessage) Marshal() ([]byte, error) { return nil, errors.New("序列化失败") }

// TestWritePumpMarshalError 序列化失败的消息被写泵跳过，后续正常消息不受影响
func TestWritePumpMarshalError(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) { return "user_marshal", nil })

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
	if !assert.NotNil(t, readUntilType(t, conn, MsgTypeAuthOK), "鉴权应成功") {
		return
	}

	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	// 坏消息入队成功（投递不序列化），随后的好消息必须正常送达
	if !assert.NoError(t, hub.SendToClient(context.Background(), "user_marshal", badMessage{})) {
		return
	}
	if !assert.NoError(t, hub.SendToClient(context.Background(), "user_marshal", NewMessage("good", nil))) {
		return
	}
	msg := readUntilType(t, conn, "good")
	if !assert.NotNil(t, msg, "序列化失败的消息不应阻塞后续消息投递") {
		return
	}
	assert.Equal(t, "good", msg["type"])
}

// TestSendWithCanceledContext 已取消 ctx 下的四类定向发送：
// select 在「通道可写」与「ctx 已取消」之间随机选择，循环 800 次两分支皆会命中；
// 断言取消分支确实返回 context.Canceled，而非把已取消的请求静默视为成功
