package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ixugo/goddd/pkg/assert"
)

func BenchmarkSendToClient(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	// 设置简单鉴权
	hub.SetAuthHandler(func(message Message) (string, error) {
		data := map[string]any{}
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return "user_" + token, nil
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 创建多个客户端连接
	numClients := 1000
	clients := make([]*websocket.Conn, numClients)
	clientIDs := make([]string, numClients)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 建立连接并鉴权
	for i := range numClients {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(b, err) {
			return
		}
		clients[i] = conn

		// 鉴权
		authMsg := map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		}
		err = conn.WriteJSON(authMsg)
		if !assert.NoError(b, err) {
			return
		}

		// 读取鉴权响应
		var response map[string]any
		err = conn.ReadJSON(&response)
		if !assert.NoError(b, err) {
			return
		}

		clientIDs[i] = fmt.Sprintf("user_%d", i)
	}

	// 等待所有连接建立
	time.Sleep(100 * time.Millisecond)

	// 准备测试消息
	testMsg := NewMessage("test", map[string]string{"data": "benchmark test"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			clientID := clientIDs[i%numClients]
			_ = hub.SendToClient(context.Background(), clientID, testMsg)
			i++
		}
	})

	// 清理连接
	for _, conn := range clients {
		conn.Close()
	}
}

func BenchmarkBroadcast(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	// 设置简单鉴权
	hub.SetAuthHandler(func(message Message) (string, error) {
		data := map[string]any{}
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}

		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return "user_" + token, nil
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 创建多个客户端连接
	numClients := 100
	clients := make([]*websocket.Conn, numClients)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 建立连接并鉴权
	for i := range numClients {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(b, err) {
			return
		}
		clients[i] = conn

		// 鉴权
		authMsg := map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		}
		err = conn.WriteJSON(authMsg)
		if !assert.NoError(b, err) {
			return
		}

		// 读取鉴权响应
		var response map[string]any
		err = conn.ReadJSON(&response)
		if !assert.NoError(b, err) {
			return
		}
	}

	// 等待所有连接建立
	time.Sleep(100 * time.Millisecond)

	// 准备测试消息
	testMsg := NewMessage("broadcast", map[string]string{"data": "benchmark broadcast"})

	for b.Loop() {
		hub.Broadcast(testMsg)
	}

	// 清理连接
	for _, conn := range clients {
		conn.Close()
	}
}

func BenchmarkSendToClientAsync(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		data := map[string]any{}
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return "user_" + token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	numClients := 1000
	clients := make([]*websocket.Conn, numClients)
	clientIDs := make([]string, numClients)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	for i := range numClients {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(b, err) {
			return
		}
		clients[i] = conn

		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		})
		if !assert.NoError(b, err) {
			return
		}

		var response map[string]any
		if !assert.NoError(b, conn.ReadJSON(&response)) {
			return
		}

		clientIDs[i] = fmt.Sprintf("user_%d", i)
	}

	time.Sleep(100 * time.Millisecond)

	testMsg := NewMessage("test", map[string]string{"data": "benchmark async"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = hub.SendToClientAsync(context.Background(), clientIDs[i%numClients], testMsg)
			i++
		}
	})

	for _, conn := range clients {
		conn.Close()
	}
}

// BenchmarkHandleMessage 压测入站消息处理链路：readPump 解析 → 路由派发 → 处理器执行。
// 处理器将消息数据绑定到强类型结构体，模拟真实业务用法。
func BenchmarkHandleMessage(b *testing.B) {
	type echoData struct {
		CPU    float64 `json:"cpu"`
		Memory float64 `json:"memory"`
		Disk   float64 `json:"disk"`
	}

	hub := NewHub()
	defer hub.Close()

	hub.Handle("echo", Wrap(func(c *Client, d echoData) error {
		return c.Send(context.Background(), NewMessage("echo", d))
	}))

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(b, err) {
		return
	}
	defer conn.Close()

	// 先发鉴权消息打通链路，读走 auth_ok 回执
	if !assert.NoError(b, conn.WriteJSON(map[string]any{"type": MsgTypeAuth})) {
		return
	}
	var authResp map[string]any
	if !assert.NoError(b, conn.ReadJSON(&authResp)) {
		return
	}

	msg := map[string]any{
		"type": "echo",
		"data": map[string]any{"cpu": 1.1, "memory": 2.2, "disk": 3.3},
	}
	var resp map[string]any

	b.ResetTimer()
	for b.Loop() {
		if !assert.NoError(b, conn.WriteJSON(msg)) {
			return
		}
		if !assert.NoError(b, conn.ReadJSON(&resp)) {
			return
		}
	}
}

func BenchmarkGetClients(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		data := map[string]any{}
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return "user_" + token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	numClients := 100
	clients := make([]*websocket.Conn, numClients)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	for i := range numClients {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(b, err) {
			return
		}
		clients[i] = conn

		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		})
		if !assert.NoError(b, err) {
			return
		}

		var response map[string]any
		if !assert.NoError(b, conn.ReadJSON(&response)) {
			return
		}
	}

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for b.Loop() {
		_ = hub.GetClients()
	}

	for _, conn := range clients {
		conn.Close()
	}
}
