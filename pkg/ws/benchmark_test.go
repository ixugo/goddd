package ws

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func BenchmarkSendToClient(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	// 设置简单鉴权
	hub.SetAuthHandler(func(message Message) (string, error) {
		data := message.Data()
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
		require.NoError(b, err)
		clients[i] = conn

		// 鉴权
		authMsg := map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		}
		err = conn.WriteJSON(authMsg)
		require.NoError(b, err)

		// 读取鉴权响应
		var response map[string]any
		err = conn.ReadJSON(&response)
		require.NoError(b, err)

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
		data := message.Data()

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
		require.NoError(b, err)
		clients[i] = conn

		// 鉴权
		authMsg := map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		}
		err = conn.WriteJSON(authMsg)
		require.NoError(b, err)

		// 读取鉴权响应
		var response map[string]any
		err = conn.ReadJSON(&response)
		require.NoError(b, err)
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
		data := message.Data()
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
		require.NoError(b, err)
		clients[i] = conn

		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		})
		require.NoError(b, err)

		var response map[string]any
		require.NoError(b, conn.ReadJSON(&response))

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

func BenchmarkGetClients(b *testing.B) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		data := message.Data()
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
		require.NoError(b, err)
		clients[i] = conn

		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": fmt.Sprintf("%d", i)},
		})
		require.NoError(b, err)

		var response map[string]any
		require.NoError(b, conn.ReadJSON(&response))
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
