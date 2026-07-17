package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
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
		data := message.Data()
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
	require.NoError(t, err)
	defer conn.Close()

	// 发送鉴权消息
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{
			"token": "valid_token",
		},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	require.NoError(t, err)

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
	require.NoError(t, err)
	defer conn.Close()

	// 发送无效鉴权消息
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{
			"token": "invalid_token",
		},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

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

	// 创建两个客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn2.Close()

	// 客户端1鉴权
	authMsg1 := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn1.WriteJSON(authMsg1)
	require.NoError(t, err)

	// 客户端2鉴权
	authMsg2 := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "456"},
	}
	err = conn2.WriteJSON(authMsg2)
	require.NoError(t, err)

	// 读取鉴权响应
	var response1, response2 map[string]any
	err = conn1.ReadJSON(&response1)
	require.NoError(t, err)
	err = conn2.ReadJSON(&response2)
	require.NoError(t, err)

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
	require.NoError(t, err)
	err = conn2.ReadJSON(&msg2)
	require.NoError(t, err)

	assert.Equal(t, "notification", msg1["type"])
	assert.Equal(t, "notification", msg2["type"])
}

func TestSendToClient(t *testing.T) {
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

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	require.NoError(t, err)

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 发送私人消息
	privateMsg := NewMessage("private", map[string]string{
		"message": "Hello user_123!",
	})
	err = hub.SendToClient(context.Background(), "user_123", privateMsg)
	require.NoError(t, err)

	// 验证客户端收到消息
	var msg map[string]any
	err = conn.ReadJSON(&msg)
	require.NoError(t, err)

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
	hub.RegisterHandler("chat", HandlerFunc(func(client *Client, message Message) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, message)
		receivedClients = append(receivedClients, client)
		mu.Unlock()
		return nil
	}))

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

	// 创建客户端连接
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	require.NoError(t, err)

	// 发送业务消息
	businessMsg := map[string]any{
		"type": "chat",
		"data": map[string]any{
			"message": "Hello world!",
		},
	}
	err = conn.WriteJSON(businessMsg)
	require.NoError(t, err)

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
		data := message.Data()
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
	require.NoError(t, err)
	defer conn.Close()

	// 鉴权
	authMsg := map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "123"},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var response map[string]any
	err = conn.ReadJSON(&response)
	require.NoError(t, err)

	// 等待连接建立
	time.Sleep(100 * time.Millisecond)

	// 验证客户端元数据
	clients := hub.GetClients()
	require.Len(t, clients, 1)

	metadata := clients[0].GetMetadata()
	assert.Contains(t, metadata, "connect_time")
	assert.Contains(t, metadata, "user_agent")
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
	require.NoError(t, err)
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
	require.NoError(t, err)
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
		data := message.Data()
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
		require.NoError(t, err)
		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": userID},
		})
		require.NoError(t, err)
		var resp map[string]any
		err = conn.ReadJSON(&resp)
		require.NoError(t, err)
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
	require.NoError(t, err)

	// 旧连接应收到 force_logout
	var msg map[string]any
	err = oldConn.ReadJSON(&msg)
	require.NoError(t, err)
	assert.Equal(t, "force_logout", msg["type"])
	data := msg["data"].(map[string]any)
	assert.Equal(t, "same_device_type_login", data["reason"])
}

// TestForceLogoutMultiTab 同一用户多标签页都能收到 force_logout
func TestForceLogoutMultiTab(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		data := message.Data()
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
		require.NoError(t, err)
		err = conn.WriteJSON(map[string]any{
			"type": MsgTypeAuth,
			"data": map[string]any{"token": userID},
		})
		require.NoError(t, err)
		var resp map[string]any
		err = conn.ReadJSON(&resp)
		require.NoError(t, err)
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
	require.NoError(t, err)

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
	require.NoError(t, err)
	defer conn.Close()

	authMsg := map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}}
	require.NoError(t, conn.WriteJSON(authMsg))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	// 再次鉴权
	require.NoError(t, conn.WriteJSON(authMsg))
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	time.Sleep(100 * time.Millisecond)

	// 仅登记一次
	clients := hub.GetClients()
	require.Len(t, clients, 1)

	// 定向投递仅送达一份
	require.NoError(t, hub.SendToClient(context.Background(), "user_reauth", NewMessage("once", nil)))
	require.NoError(t, conn.ReadJSON(&resp))
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
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	require.Eventually(t, func() bool {
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
	require.NoError(t, c.Send(context.Background(), NewMessage("m1", nil)))

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
	require.NoError(t, err)

	require.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 50*time.Millisecond)

	conn.Close()
	require.Eventually(t, func() bool {
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
	require.NoError(t, err)
	defer conn.Close()

	// 发送远超 64 字节的消息
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, make([]byte, 256)))

	// 连接应被服务端关闭
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp map[string]any
	err = conn.ReadJSON(&resp)
	assert.Error(t, err)
}

// TestBroadcastSkipsSlowConsumer 广播跳过发送队列满的慢连接，但不剔除、不关闭
func TestBroadcastSkipsSlowConsumer(t *testing.T) {
	hub := NewHub(func(c *Config) {
		c.MessageQueueSize = 1
	})
	defer hub.Close()

	// 构造无网络假客户端：队列满且无人消费
	c := &Client{send: make(chan Message, 1), isAuth: true, id: "slow"}
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

// TestConcurrentSendAndClose 并发 Send 与连接关闭不得产生 panic 或数据竞争（配合 -race）
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
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 50*time.Millisecond)

	c := hub.GetClients()[0]
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 100 {
				_ = c.Send(context.Background(), NewMessage("spam", j))
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	_ = c.close()
	wg.Wait()
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
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth, "data": map[string]any{"token": "t"}}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))

	time.Sleep(100 * time.Millisecond)

	err = hub.SendToClientAsync(context.Background(), "user_async", NewMessage("async_msg", nil))
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, conn.ReadJSON(&resp))
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
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]any{"type": MsgTypeAuth}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Equal(t, MsgTypeAuthOK, resp["type"])

	require.Eventually(t, func() bool {
		clients := hub.GetClients()
		if len(clients) != 1 {
			return false
		}
		_, err := uuid.Parse(clients[0].ID())
		return err == nil
	}, 2*time.Second, 50*time.Millisecond, "空 ID 应保留 UUID")
}

// groupTestClient 拨号并完成鉴权，返回连接
func groupTestClient(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	require.NoError(t, conn.WriteJSON(map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": token},
	}))
	var resp map[string]any
	require.NoError(t, conn.ReadJSON(&resp))
	require.Equal(t, MsgTypeAuthOK, resp["type"])
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
		token, _ := message.Data()["token"].(string)
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

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 3
	}, 2*time.Second, 20*time.Millisecond)

	findClientByID(t, hub, "user_a").JoinGroup("room1")
	findClientByID(t, hub, "user_b").JoinGroup("room1")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 2
	}, 2*time.Second, 20*time.Millisecond)

	err := hub.SendToGroup(context.Background(), "room1", NewMessage("room_msg", nil))
	require.NoError(t, err)

	for _, conn := range []*websocket.Conn{connA, connB} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg map[string]any
		require.NoError(t, conn.ReadJSON(&msg))
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

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_solo")
	client.JoinGroup("room1")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 1
	}, 2*time.Second, 20*time.Millisecond)

	client.LeaveGroup("room1")
	client.LeaveGroup("room1")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 0
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, hub.SendToGroup(context.Background(), "room1", NewMessage("room_msg", nil)))
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

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_tmp")
	client.JoinGroup("room1")
	client.JoinGroup("room2")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 1 && hub.GroupSize("room2") == 1
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, conn.Close())
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 0 && hub.GroupSize("room2") == 0
	}, 2*time.Second, 20*time.Millisecond, "断开后应自动清出所有分组")
}

// TestGroupSlowConsumerSkipped 组内慢连接跳过：发送队列积压时不阻塞投递，正常成员照收
func TestGroupSlowConsumerSkipped(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	hub.SetAuthHandler(func(message Message) (string, error) {
		token, _ := message.Data()["token"].(string)
		return "user_" + token, nil
	})

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	slow := groupTestClient(t, wsURL, "slow")
	defer slow.Close()
	fast := groupTestClient(t, wsURL, "fast")
	defer fast.Close()

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 2
	}, 2*time.Second, 20*time.Millisecond)

	slowClient := findClientByID(t, hub, "user_slow")
	fastClient := findClientByID(t, hub, "user_fast")
	slowClient.JoinGroup("room1")
	fastClient.JoinGroup("room1")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room1") == 2
	}, 2*time.Second, 20*time.Millisecond)

	// 填满慢连接的发送队列，模拟消费积压
	for range hub.config.MessageQueueSize {
		slowClient.send <- NewMessage("flood", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.SendToGroup(ctx, "room1", NewMessage("room_msg", nil)))

	fast.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	require.NoError(t, fast.ReadJSON(&msg))
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

	require.Eventually(t, func() bool {
		return len(hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	client := findClientByID(t, hub, "user_edge")

	// 空组名为空操作
	client.JoinGroup("")
	client.LeaveGroup("")
	assert.Equal(t, 0, hub.GroupSize(""))

	// 向不存在的分组投递视为成功
	require.NoError(t, hub.SendToGroup(context.Background(), "no_such_room", NewMessage("m", nil)))

	// 异步投递
	client.JoinGroup("room_async")
	require.Eventually(t, func() bool {
		return hub.GroupSize("room_async") == 1
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, hub.SendToGroupAsync(context.Background(), "room_async", NewMessage("async_room", nil)))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]any
	require.NoError(t, conn.ReadJSON(&msg))
	assert.Equal(t, "async_room", msg["type"])
}

// TestJoinSkipsDeadClient 连接在 join 事件排队期间已断开时不予登记，防止幽灵连接
func TestJoinSkipsDeadClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	dead := newClient(nil, hub, nil)
	dead.cancel() // 模拟 join 入队后连接即断开的重排窗口

	hub.join <- dead
	require.Never(t, func() bool {
		return len(hub.GetClients()) > 0
	}, 300*time.Millisecond, 50*time.Millisecond, "已死连接不应登记")
}
