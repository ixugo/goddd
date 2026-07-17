package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	defer conn.Close()

	// 读取欢迎消息
	var welcomeMsg map[string]any
	err = conn.ReadJSON(&welcomeMsg)
	require.NoError(t, err)

	assert.Equal(t, "welcome", welcomeMsg["type"])
	data := welcomeMsg["data"].(map[string]any)
	assert.Contains(t, data["message"], "连接成功")
}

func TestAuthentication(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 读取欢迎消息
	var welcomeMsg map[string]any
	err = conn.ReadJSON(&welcomeMsg)
	require.NoError(t, err)

	// 发送正确的鉴权消息
	authMsg := map[string]any{
		"type": "auth",
		"data": map[string]any{
			"token": "a67c2bacf5c691b6",
		},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var authResponse map[string]any
	err = conn.ReadJSON(&authResponse)
	require.NoError(t, err)

	assert.Equal(t, "auth_ok", authResponse["type"])
}

func TestAuthenticationFailure(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 读取欢迎消息
	var welcomeMsg map[string]any
	err = conn.ReadJSON(&welcomeMsg)
	require.NoError(t, err)

	// 发送错误的鉴权消息
	authMsg := map[string]any{
		"type": "auth",
		"data": map[string]any{
			"token": "invalid_token",
		},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 尝试读取错误响应，如果连接被关闭则跳过
	var errorResponse map[string]any
	err = conn.ReadJSON(&errorResponse)
	if err != nil {
		// 连接可能因为鉴权失败而被关闭，这是正常的
		if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
			t.Log("连接因鉴权失败被关闭，这是预期行为")
			return
		}
		require.NoError(t, err)
	}

	assert.Equal(t, "error", errorResponse["type"])
}

func TestInvalidMessageType(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	// 连接并鉴权
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// 读取欢迎消息
	var welcomeMsg map[string]any
	err = conn.ReadJSON(&welcomeMsg)
	require.NoError(t, err)

	// 鉴权
	authMsg := map[string]any{
		"type": "auth",
		"data": map[string]any{
			"token": "a67c2bacf5c691b6",
		},
	}
	err = conn.WriteJSON(authMsg)
	require.NoError(t, err)

	// 读取鉴权响应
	var authResponse map[string]any
	err = conn.ReadJSON(&authResponse)
	require.NoError(t, err)

	// 发送无效消息类型
	invalidMsg := map[string]any{
		"type": "device_message",
		"data": map[string]any{
			"type": 999, // 无效的消息类型
		},
	}
	err = conn.WriteJSON(invalidMsg)
	require.NoError(t, err)

	// 读取响应（ErrorMessage 格式: {"type":"error","msg":"..."}）
	var response map[string]any
	err = conn.ReadJSON(&response)
	require.NoError(t, err)

	assert.Equal(t, "error", response["type"])
	msg, _ := response["msg"].(string)
	assert.Contains(t, msg, "未知的消息类型")
}

func TestConcurrentConnections(t *testing.T) {
	server := createTestServer()
	defer server.Close()

	const numClients = 5
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"

	// 创建多个并发连接
	for i := range numClients {
		go func(clientID int) {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Errorf("客户端 %d 连接失败: %v", clientID, err)
				return
			}
			defer conn.Close()

			// 读取欢迎消息
			var welcomeMsg map[string]any
			err = conn.ReadJSON(&welcomeMsg)
			if err != nil {
				t.Errorf("客户端 %d 读取欢迎消息失败: %v", clientID, err)
				return
			}

			// 鉴权
			authMsg := map[string]any{
				"type": "auth",
				"data": map[string]any{
					"token": "a67c2bacf5c691b6",
				},
			}
			err = conn.WriteJSON(authMsg)
			if err != nil {
				t.Errorf("客户端 %d 发送鉴权消息失败: %v", clientID, err)
				return
			}

			// 读取鉴权响应
			var authResponse map[string]any
			err = conn.ReadJSON(&authResponse)
			if err != nil {
				t.Errorf("客户端 %d 读取鉴权响应失败: %v", clientID, err)
				return
			}

			// 发送测试消息
			testMsg := map[string]any{
				"type": "device_message",
				"data": map[string]any{
					"type": 1,
					"cpu":  float64(clientID * 10),
				},
			}
			err = conn.WriteJSON(testMsg)
			if err != nil {
				t.Errorf("客户端 %d 发送测试消息失败: %v", clientID, err)
				return
			}

			// 读取响应
			var response map[string]any
			err = conn.ReadJSON(&response)
			if err != nil {
				t.Errorf("客户端 %d 读取响应失败: %v", clientID, err)
				return
			}
		}(i)
	}

	// 等待所有连接完成
	time.Sleep(2 * time.Second)
}
