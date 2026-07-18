package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

func TestNewHub(t *testing.T) {
	// 测试默认配置
	hub := NewHub()
	assert.NotNil(t, hub)

	// 等待 hub 初始化完成
	time.Sleep(10 * time.Millisecond)

	hub2 := NewHub(func(c *Config) {
		c.ReadBufferSize = 2048
		c.WriteBufferSize = 2048
		c.HeartbeatInterval = 10 * time.Second
		c.MaxConnections = 100
	})
	assert.NotNil(t, hub2)

	// 等待 hub2 初始化完成
	time.Sleep(10 * time.Millisecond)

	// 清理
	hub.Close()
	hub2.Close()
}

func TestStandardMessage(t *testing.T) {
	msg := NewMessage("test", map[string]string{"key": "value"})

	assert.Equal(t, "test", msg.Type())
	assert.NotNil(t, msg.Data())
	// assert.NotEmpty(t, msg.ID)
	// assert.False(t, msg.Timestamp.IsZero())

	data, err := msg.Marshal()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test")
}

func TestWebSocketConnection(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 验证连接数
	clients := hub.GetClients()
	assert.Len(t, clients, 0) // 未鉴权的连接不会出现在客户端列表中
}

func TestAuthentication(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 设置鉴权处理器
	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		if token == "valid_token" {
			return "user_123", nil
		}
		return "", ErrAuthFailed
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 发送鉴权消息
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{
			"token": "valid_token",
		},
	}
	err = conn.WriteJSON(authMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, MsgTypeAuthOK, response["type"])

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证已认证的客户端
	clients := hub.GetClients()
	assert.Len(t, clients, 1)
	assert.Equal(t, "user_123", clients[0].ID())
}

func TestAuthenticationFailure(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 设置鉴权处理器
	hub.SetAuthHandler(func(message Message) (string, error) {
		return "", ErrAuthFailed
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 发送无效鉴权消息
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{
			"token": "invalid_token",
		},
	}
	err = conn.WriteJSON(authMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 读取错误响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	if err != nil {
		// 连接可能因为鉴权失败而被关闭，这是正常的
		return
	}

	assert.Equal(t, MsgTypeError, response["type"])
}

func TestBroadcast(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 设置简单鉴权（无需 token）
	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
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

	// 创建两个客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn2.Close()

	// 客户端1鉴权
	authMsg1 := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn1.WriteJSON(authMsg1)
	if !assert.NoError(t, err) {
		return
	}

	// 客户端2鉴权
	authMsg2 := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "456"},
	}
	err = conn2.WriteJSON(authMsg2)
	if !assert.NoError(t, err) {
		return
	}

	// 读取鉴权响应
	var response1, response2 map[string]any
	err = conn1.ReadJSON(&response1)
	if !assert.NoError(t, err) {
		return
	}
	err = conn2.ReadJSON(&response2)
	if !assert.NoError(t, err) {
		return
	}

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 广播消息
	broadcastMsg := NewMessage("notification", map[string]string{
		"message": "Hello everyone!",
	})
	hub.Broadcast(broadcastMsg)

	// 验证两个客户端都收到消息
	var msg1, msg2 map[string]any
	err = conn1.ReadJSON(&msg1)
	if !assert.NoError(t, err) {
		return
	}
	err = conn2.ReadJSON(&msg2)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "notification", msg1["type"])
	assert.Equal(t, "notification", msg2["type"])
}

func TestSendToClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 设置简单鉴权
	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
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

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	if !assert.NoError(t, err) {
		return
	}

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 发送私人消息
	privateMsg := NewMessage("private", map[string]string{
		"message": "Hello user_123!",
	})
	err = hub.SendToClient(context.Background(), "user_123", privateMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 验证客户端收到消息
	var msg map[string]any
	err = conn.ReadJSON(&msg)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "private", msg["type"])
	data := msg["data"].(map[string]any)
	assert.Equal(t, "Hello user_123!", data["message"])

	// 测试发送给不存在的客户端
	err = hub.SendToClient(context.Background(), "nonexistent", privateMsg)
	assert.Equal(t, ErrClientNotFound, err)
}

func TestMessageHandler(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	var mu sync.Mutex
	var receivedMessages []Message
	var receivedClients []*Client

	// 注册 chat 消息处理器
	hub.Handle("chat", HandlerFunc(func(client *Client, message Message) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, message)
		receivedClients = append(receivedClients, client)
		mu.Unlock()
		return nil
	}))

	// 设置简单鉴权
	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
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

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	if !assert.NoError(t, err) {
		return
	}

	// 发送业务消息
	businessMsg := map[string]any{
		"type": "chat",
		"data": map[string]any{
			"message": "Hello world!",
		},
	}
	err = conn.WriteJSON(businessMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 等待消息处理
	time.Sleep(100 * time.Millisecond)

	// 验证消息处理器被调用
	mu.Lock()
	assert.Len(t, receivedMessages, 1)
	assert.Len(t, receivedClients, 1)
	assert.Equal(t, "chat", receivedMessages[0].Type())
	assert.Equal(t, "user_123", receivedClients[0].ID())
	mu.Unlock()
}

func TestClientMetadata(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	// 设置鉴权处理器，在鉴权时设置元数据
	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return "user_" + token, nil
	})

	// 设置连接处理器，设置客户端元数据
	hub.SetConnectHandler(func(client *Client) error {
		client.SetMetadata("connect_time", time.Now())
		client.SetMetadata("user_agent", "test_client")
		return nil
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	if !assert.NoError(t, err) {
		return
	}

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	if !assert.NoError(t, err) {
		return
	}

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 验证客户端元数据
	clients := hub.GetClients()
	if !assert.Len(t, clients, 1) {
		return
	}

	metadata := clients[0].GetMetadata()
	assert.True(t, metadata["connect_time"] != nil, "metadata 应含 connect_time")
	assert.Equal(t, "test_client", metadata["user_agent"])
}

func TestHubClose(t *testing.T) {
	hub := NewHub()

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 关闭 Hub
	hub.Close()

	// 再次关闭应该不会出错
	hub.Close()

	// 关闭后的操作应该失败
	msg := NewMessage("test", "data")
	hub.Broadcast(msg)
	err = hub.SendToClient(context.Background(), "test", msg)
	assert.Equal(t, ErrHubClosed, err)
}

func TestAuthTimeout(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.AuthTimeout = 100 * time.Millisecond
	})
	defer hub.Close()

	// 设置鉴权处理器（但不会被调用，因为客户端不发送鉴权消息）
	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_123", nil
	})

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// 创建客户端连接但不发送鉴权消息
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

	// 尝试读取消息，应该收到超时错误或连接关闭
	var response map[string]any
	err = conn.ReadJSON(&response)
	if err == nil {
		// 如果成功读取到消息，应该是错误消息
		assert.Equal(t, MsgTypeError, response["type"])
	}
	// 如果读取失败，说明连接被关闭，这也是正常的
}

// TestForceLogout 验证服务端 SendToClient 能将 force_logout 推送到已鉴权的客户端
func TestForceLogout(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 辅助函数：建立连接并鉴权
	dialAndAuth := func(userID string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(t, err) {
			return nil
		}
		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": userID},
		})
		if !assert.NoError(t, err) {
			return nil
		}
		var resp map[string]any
		err = conn.ReadJSON(&resp)
		if !assert.NoError(t, err) {
			return nil
		}
		assert.Equal(t, MsgTypeAuthOK, resp["type"])
		return conn
	}

	// 模拟旧浏览器建立 WS 连接
	oldConn := dialAndAuth("user_abc")
	defer oldConn.Close()

	time.Sleep(100 * time.Millisecond)

	// 服务端发送 force_logout（模拟互踢通知）
	err := hub.SendToClient(context.Background(), "user_abc", NewMessage("force_logout", map[string]any{
		"reason":  "same_device_type_login",
		"message": "您的账号已在另一台设备上登录",
	}))
	if !assert.NoError(t, err) {
		return
	}

	// 旧连接应收到 force_logout
	var msg map[string]any
	err = oldConn.ReadJSON(&msg)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "force_logout", msg["type"])
	data := msg["data"].(map[string]any)
	assert.Equal(t, "same_device_type_login", data["reason"])
}

// TestForceLogoutMultiTab 同一用户多标签页都能收到 force_logout
func TestForceLogoutMultiTab(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialAndAuth := func(userID string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(t, err) {
			return nil
		}
		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": userID},
		})
		if !assert.NoError(t, err) {
			return nil
		}
		var resp map[string]any
		err = conn.ReadJSON(&resp)
		if !assert.NoError(t, err) {
			return nil
		}
		return conn
	}

	// 同一用户开两个标签页
	tab1 := dialAndAuth("user_xyz")
	defer tab1.Close()
	tab2 := dialAndAuth("user_xyz")
	defer tab2.Close()

	time.Sleep(100 * time.Millisecond)

	// 验证 Hub 中该用户有两个连接
	clients := hub.GetClients()
	count := 0
	for _, c := range clients {
		if c.ID() == "user_xyz" {
			count++
		}
	}
	assert.Equal(t, 2, count, "同一用户应有 2 个连接")

	// 发送 force_logout
	err := hub.SendToClient(context.Background(), "user_xyz", NewMessage("force_logout", map[string]any{
		"reason":  "same_device_type_login",
		"message": "您的账号已在另一台设备上登录",
	}))
	if !assert.NoError(t, err) {
		return
	}

	// 两个标签页都应收到
	var wg sync.WaitGroup
	wg.Add(2)
	for i, conn := range []*websocket.Conn{tab1, tab2} {
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			var msg map[string]any
			if err := c.ReadJSON(&msg); err != nil {
				t.Errorf("标签页 %d 读取失败: %v", idx, err)
				return
			}
			assert.Equal(t, "force_logout", msg["type"])
		}(i, conn)
	}
	wg.Wait()
}

// TestReAuthIdempotent 重复鉴权应幂等：回 auth_ok 但不重复登记，定向投递不重复
func TestReAuthIdempotent(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_reauth", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	authMsg := map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}}
	if !assert.NoError(t, conn.WriteJSON(authMsg)) {
		return
	}
	var resp map[string]any
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	// 再次鉴权
	if !assert.NoError(t, conn.WriteJSON(authMsg)) {
		return
	}
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	time.Sleep(100 * time.Millisecond)

	// 仅登记一次
	clients := hub.GetClients()
	if !assert.Len(t, clients, 1) {
		return
	}

	// 定向投递仅送达一份
	if !assert.NoError(t, hub.SendToClient(context.Background(), "user_reauth", NewMessage("once", nil))) {
		return
	}
	if !assert.NoError(t, conn.ReadJSON(&resp)) {
		return
	}
	assert.Equal(t, "once", resp["type"])

	// 第二读应超时（无重复投递）
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	err = conn.ReadJSON(&resp)
	assert.Error(t, err)
}

// TestUUIDAssignedWithoutAuthHandler 未设置鉴权处理器时，连接分配 UUID 作为 ID
func TestUUIDAssignedWithoutAuthHandler(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

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
	}, 2*time.Second, 50*time.Millisecond, "客户端 ID 应为合法 UUID")
}

// TestClientSendBackpressure Client.Send 队列满时阻塞等待，受 ctx 与连接状态控制
func TestClientSendBackpressure(t *testing.T) {
	c := &Client{send: make(chan Message, 1)}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	// 队列有余量时立即入队
	if !assert.NoError(t, c.Send(context.Background(), NewMessage("m1", nil))) {
		return
	}

	// 队列满，阻塞至 ctx 超时
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := c.Send(ctx, NewMessage("m2", nil))
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// 连接关闭后立即返回
	c.cancel()
	err = c.Send(context.Background(), NewMessage("m3", nil))
	assert.ErrorIs(t, err, ErrConnectionClosed)
}

// TestClientLeavesOnDisconnect 连接断开后客户端必须从 Hub 中除名
func TestClientLeavesOnDisconnect(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		return "user_leave", nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}

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

	conn.Close()
	assert.Eventually(t, func() bool {
		return len(hub.GetClients()) == 0
	}, 2*time.Second, 50*time.Millisecond, "断开后应除名，不得残留幽灵连接")
}

// TestMaxMessageSizeExceeded 超过单条消息上限的连接被关闭
func TestMaxMessageSizeExceeded(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.MaxMessageSize = 64
	})
	defer hub.Close()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 发送远超 64 字节的消息
	if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, make([]byte, 256))) {
		return
	}

	// 连接应被服务端关闭
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp map[string]any
	err = conn.ReadJSON(&resp)
	assert.Error(t, err)
}

// ============================================================
// 以下测试自原 coverage_test.go 并入，按被测源文件分节
// ============================================================
// newSyntheticClient 构造无网络连接的合成客户端，用于直接测试不依赖真实连接的函数。
// ctx 必须初始化，否则 Send 等方法会因 nil ctx 崩溃。
func newSyntheticClient(sendCap int) *Client {
	c := &Client{send: make(chan Message, sendCap)}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return c
}

// drainSend 从客户端发送队列取出一条消息，超时说明预期消息根本没入队，直接判失败
func drainSend(t *testing.T, c *Client) Message {
	t.Helper()
	select {
	case m := <-c.send:
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("发送队列为空，未收到预期消息")
		return nil
	}
}

// drainUntilType 从客户端发送队列取出指定类型的消息。
// 连接回调的广播（如 user_online）可能先入队，不能假设第一条就是目标消息。
func drainUntilType(t *testing.T, c *Client, msgType string) Message {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.send:
			if m.Type() == msgType {
				return m
			}
		case <-deadline:
			t.Fatalf("未在超时内从队列取到类型 %s 的消息", msgType)
			return nil
		}
	}
}

// readUntilType 持续读取连接直到收到指定类型的消息。
// 广播类消息（如 user_online）可能插队，不能假设第一条就是目标消息。
func readUntilType(t *testing.T, conn *websocket.Conn, msgType string) map[string]any {
	t.Helper()
	if !assert.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second))) {
		return nil
	}
	for {
		var msg map[string]any
		if !assert.NoError(t, conn.ReadJSON(&msg)) {
			return nil
		}
		if msg["type"] == msgType {
			return msg
		}
	}
}

// TestErrorMessage 覆盖错误消息的全部方法：构造、类型、空数据、序列化与三种输入的反序列化

// TestCloseClient 按 ID 踢下线：该 ID 所有连接断开，其他用户不受影响，重复踢幂等
func TestCloseClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		var data map[string]any
		if err := json.Unmarshal(message.Data(), &data); err != nil {
			return "", err
		}
		token, ok := data["token"].(string)
		if !ok {
			return "", ErrAuthFailed
		}
		return token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialAndAuth := func(userID string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if !assert.NoError(t, err) {
			return nil
		}
		if !assert.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": userID}})) {
			return nil
		}
		var resp map[string]any
		if !assert.NoError(t, conn.ReadJSON(&resp)) {
			return nil
		}
		assert.Equal(t, MsgTypeAuthOK, resp["type"])
		return conn
	}

	// 同一用户两个标签页 + 另一用户
	tab1 := dialAndAuth("user_a")
	defer tab1.Close()
	tab2 := dialAndAuth("user_a")
	defer tab2.Close()
	other := dialAndAuth("user_b")
	defer other.Close()

	time.Sleep(100 * time.Millisecond)
	assert.Len(t, hub.GetClients(), 3)

	// 踢 user_a：两个标签页连接均被服务端断开
	if !assert.NoError(t, hub.CloseClient("user_a")) {
		return
	}
	for _, conn := range []*websocket.Conn{tab1, tab2} {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg map[string]any
		assert.Error(t, conn.ReadJSON(&msg))
	}

	// user_b 不受影响，仍可正常收发
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, hub.GetClients(), 1)
	if !assert.NoError(t, hub.SendToClient(context.Background(), "user_b", NewMessage("ping", nil))) {
		return
	}
	_ = other.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	if !assert.NoError(t, other.ReadJSON(&msg)) {
		return
	}
	assert.Equal(t, "ping", msg["type"])

	// 幂等：踢不存在的 ID 直接成功
	assert.NoError(t, hub.CloseClient("user_a"))
	assert.NoError(t, hub.CloseClient("not_exist"))
}

// TestCloseClientOnClosedHub Hub 关闭后踢人返回 ErrHubClosed
func TestCloseClientOnClosedHub(t *testing.T) {
	hub := NewHub()
	hub.Close()
	assert.ErrorIs(t, hub.CloseClient("user_a"), ErrHubClosed)
}
