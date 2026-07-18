package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

func TestErrorMessage(t *testing.T) {
	msg := NewErrorMessage("出错了")
	assert.Equal(t, MsgTypeError, msg.Type())
	assert.Empty(t, msg.Data(), "错误消息无业务数据")

	data, err := msg.Marshal()
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, string(data), "出错了")

	// 反序列化可还原
	var back ErrorMessage
	if !assert.NoError(t, json.Unmarshal(data, &back)) {
		return
	}
	assert.Equal(t, "出错了", back.Msg)

	// 类型不符的 JSON 输入必然反序列化失败
	assert.Error(t, json.Unmarshal([]byte("123"), &back))
}

// TestStandardMessageExtra 覆盖标准消息的 Data 与序列化往返
func TestStandardMessageExtra(t *testing.T) {
	// Payload 序列化为原始字节返回
	m := NewMessage("t", map[string]any{"k": "v"})
	assert.Equal(t, `{"k":"v"}`, string(m.Data()))

	// 非标量以外的 Payload 同样序列化为原始字节
	m2 := NewMessage("t", "字符串载荷")
	assert.Equal(t, `"字符串载荷"`, string(m2.Data()))

	// 无 Payload 时返回 nil
	assert.Empty(t, NewMessage("t", nil).Data())

	// 反序列化可还原
	raw, err := m.Marshal()
	if !assert.NoError(t, err) {
		return
	}
	var back StandardMessage
	if !assert.NoError(t, json.Unmarshal(raw, &back)) {
		return
	}
	assert.Equal(t, "t", back.Type())
	assert.Error(t, json.Unmarshal([]byte("3.14"), &back), "类型不符的 JSON 输入必须报错")
}

// TestMessageRouter 覆盖路由器的注册、回退默认处理器与自定义默认处理器
func TestMessageRouter(t *testing.T) {
	r := NewMessageRouter()

	// 未注册类型回退到内置默认处理器：向客户端队列投递一条错误消息
	c := newSyntheticClient(1)
	handler := r.GetHandler("不存在的类型")
	if !assert.NoError(t, handler.Handle(c, NewMessage("不存在的类型", nil))) {
		return
	}
	got := drainSend(t, c)
	assert.Equal(t, MsgTypeError, got.Type())

	// 注册后精确命中
	called := false
	r.RegisterHandler("hit", HandlerFunc(func(client *Client, message Message) error {
		called = true
		return nil
	}))
	if !assert.NoError(t, r.GetHandler("hit").Handle(c, NewMessage("hit", nil))) {
		return
	}
	assert.True(t, called)

	// 自定义默认处理器接管未注册类型
	custom := false
	r.SetDefaultHandler(HandlerFunc(func(client *Client, message Message) error {
		custom = true
		return nil
	}))
	if !assert.NoError(t, r.GetHandler("其他").Handle(c, NewMessage("其他", nil))) {
		return
	}
	assert.True(t, custom)
}

// TestClientRequest 验证 Request 访问器：真实连接携带升级请求，合成客户端为 nil

func TestMessageHandlerPanic(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	var mu sync.Mutex
	var handled []string
	hub.Handle("chat", HandlerFunc(func(client *Client, message Message) error {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return err
		}
		if data["message"] == "boom" {
			panic("business panic")
		}
		mu.Lock()
		handled = append(handled, data["message"].(string))
		mu.Unlock()
		return nil
	}))

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 未设置鉴权处理器时发送 auth 消息即视为鉴权成功
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	var authResp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&authResp)) {
		return
	}

	// 先发触发 panic 的消息，再发正常消息
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": "chat", "data": map[string]any{"message": "boom"}})) {
		return
	}
	if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": "chat", "data": map[string]any{"message": "ok"}})) {
		return
	}

	time.Sleep(200 * time.Millisecond)

	// panic 未打断读循环，后续消息被正常处理
	mu.Lock()
	assert.Equal(t, []string{"ok"}, handled)
	mu.Unlock()

	// 连接仍然存活，可继续收发
	assert.NoError(t, conn.WriteJSON(map[string]any{"type": "chat", "data": map[string]any{"message": "still_alive"}}))
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, []string{"ok", "still_alive"}, handled)
	mu.Unlock()
}

// TestLifecycleCallbackPanic connect/disconnect 回调 panic 时 Hub 照常运转
