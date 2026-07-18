package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

// 创建测试服务器
func createTestServer() *httptest.Server {
	// 重新初始化 hub
	initHub()

	mux := http.NewServeMux()

	// WebSocket 服务
	mux.Handle("/websocket", hub)

	// 内嵌的 HTML 页面服务
	mux.Handle("/websocket.html", http.FileServer(http.FS(websocketHTML)))

	// 根路径重定向
	mux.HandleFunc("/", handleRoot)

	return httptest.NewServer(mux)
}

// 初始化 hub 的辅助函数
func initHub() {
	hub = createHub()
}

func TestRootRedirect(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	// 创建不跟随重定向的客户端
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(server.URL + "/")
	if !assert.NoError(t, err) {
		return
	}
	defer resp.Body.Close()

	// 检查重定向状态码
	assert.Equal(t, http.StatusFound, resp.StatusCode)

	// 检查重定向位置
	location := resp.Header.Get("Location")
	assert.Equal(t, "/websocket.html", location)
}

func TestWebSocketConnection(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	// 读取欢迎消息
	var welcomeMsg map[string]any
	if !assert.NoError(t, conn.ReadJSON(&welcomeMsg)) {
		return
	}

	assert.Equal(t, "welcome", welcomeMsg["type"])
	data := welcomeMsg["data"].(map[string]any)
	assert.Contains(t, data["message"], "连接成功")
}

// dialAndReadWelcome 拨号并读完欢迎消息，失败时标记测试失败并返回 nil
func dialAndReadWelcome(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return nil
	}
	var welcomeMsg map[string]any
	if !assert.NoError(t, conn.ReadJSON(&welcomeMsg)) {
		conn.Close()
		return nil
	}
	return conn
}

// writeAuth 发送鉴权消息并读回响应，返回响应与是否成功
func writeAuth(t *testing.T, conn *websocket.Conn, token string) map[string]any {
	t.Helper()
	authMsg := map[string]any{
		"type": "auth",
		"data": map[string]any{
			"token": token,
		},
	}
	if !assert.NoError(t, conn.WriteJSON(authMsg)) {
		return nil
	}
	var authResponse map[string]any
	if !assert.NoError(t, conn.ReadJSON(&authResponse)) {
		return nil
	}
	return authResponse
}

func TestAuthentication(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn := dialAndReadWelcome(t, wsURL)
	if conn == nil {
		return
	}
	defer conn.Close()

	authResponse := writeAuth(t, conn, "a67c2bacf5c691b6")
	if authResponse == nil {
		return
	}
	assert.Equal(t, "auth_ok", authResponse["type"])
}

func TestAuthenticationFailure(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn := dialAndReadWelcome(t, wsURL)
	if conn == nil {
		return
	}
	defer conn.Close()

	authMsg := map[string]any{
		"type": "auth",
		"data": map[string]any{
			"token": "invalid_token",
		},
	}
	if !assert.NoError(t, conn.WriteJSON(authMsg)) {
		return
	}

	// 尝试读取错误响应，如果连接被关闭则跳过
	var errorResponse map[string]any
	if err := conn.ReadJSON(&errorResponse); err != nil {
		// 连接可能因为鉴权失败而被关闭，这是正常的
		if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
			t.Log("连接因鉴权失败被关闭，这是预期行为")
			return
		}
		assert.NoError(t, err)
		return
	}

	assert.Equal(t, "error", errorResponse["type"])
}

func TestInvalidMessageType(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn := dialAndReadWelcome(t, wsURL)
	if conn == nil {
		return
	}
	defer conn.Close()

	if writeAuth(t, conn, "a67c2bacf5c691b6") == nil {
		return
	}

	// 发送无效消息类型
	invalidMsg := map[string]any{
		"type": "device_message",
		"data": map[string]any{
			"type": 999, // 无效的消息类型
		},
	}
	if !assert.NoError(t, conn.WriteJSON(invalidMsg)) {
		return
	}

	// 读取响应（ErrorMessage 格式: {"type":"error","msg":"..."}）
	var response map[string]any
	if !assert.NoError(t, conn.ReadJSON(&response)) {
		return
	}

	assert.Equal(t, "error", response["type"])
	msg, _ := response["msg"].(string)
	assert.Contains(t, msg, "未知的消息类型")
}

// TestConcurrentConnections 五个客户端并发完成鉴权与消息收发，
// 每个客户端都必须拿到正确的鉴权响应与消息回包，用 WaitGroup 等待全部完成后统一断言
func TestConcurrentConnections(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	const numClients = 5
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"

	var wg sync.WaitGroup
	for i := range numClients {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn := dialAndReadWelcome(t, wsURL)
			if conn == nil {
				return
			}
			defer conn.Close()

			authResponse := writeAuth(t, conn, "a67c2bacf5c691b6")
			if authResponse == nil {
				return
			}
			assert.Equal(t, "auth_ok", authResponse["type"], "客户端 %d 鉴权响应", clientID)

			// 发送测试消息
			testMsg := map[string]any{
				"type": "device_message",
				"data": map[string]any{
					"type": 1,
					"cpu":  float64(clientID * 10),
				},
			}
			if !assert.NoError(t, conn.WriteJSON(testMsg), "客户端 %d 发送测试消息", clientID) {
				return
			}

			// 读取响应，必须收到回包才算链路完整
			var response map[string]any
			if !assert.NoError(t, conn.ReadJSON(&response), "客户端 %d 读取响应", clientID) {
				return
			}
			assert.NotEmpty(t, response["type"], "客户端 %d 响应应有消息类型", clientID)
		}(i)
	}
	wg.Wait()
}
