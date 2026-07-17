package ws

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ExampleChatServer 示例聊天服务器
type ExampleChatServer struct {
	hub   Huber
	users map[string]string // token -> username 映射
}

// NewExampleChatServer 创建示例聊天服务器
func NewExampleChatServer() *ExampleChatServer {
	server := &ExampleChatServer{
		hub: NewHub(func(c *Config) {
			c.HeartbeatInterval = 30 * time.Second
			c.HeartbeatTimeout = 90 * time.Second
			c.AuthTimeout = 10 * time.Second
			c.MaxConnections = 100
		}),
		users: make(map[string]string),
	}

	// 模拟用户数据
	server.users["token123"] = "Alice"
	server.users["token456"] = "Bob"
	server.users["token789"] = "Charlie"

	// 设置回调处理器
	server.setupHandlers()

	return server
}

func (s *ExampleChatServer) setupHandlers() {
	// 鉴权处理器
	s.hub.SetAuthHandler(func(message Message) (string, error) {
		data := message.Data()

		token, ok := data["token"].(string)
		if !ok {
			return "", fmt.Errorf("token 不能为空")
		}

		username, exists := s.users[token]
		if !exists {
			return "", fmt.Errorf("无效的 token")
		}

		return username, nil
	})

	// 连接处理器
	s.hub.SetConnectHandler(func(client *Client) error {
		metadata := client.GetMetadata()
		username := metadata["username"]
		slog.Info("用户已连接", "username", username)

		// 广播用户上线消息
		s.hub.Broadcast(NewMessage("user_online", map[string]any{
			"username": username,
			"message":  fmt.Sprintf("%s 加入了聊天室", username),
		}))

		return nil
	})

	// 断开连接处理器
	s.hub.SetDisconnectHandler(func(client *Client, err error) {
		metadata := client.GetMetadata()
		username := metadata["username"]
		slog.Info("用户已断开连接", "username", username, "error", err)

		// 广播用户下线消息
		s.hub.Broadcast(NewMessage("user_offline", map[string]any{
			"username": username,
			"message":  fmt.Sprintf("%s 离开了聊天室", username),
		}))
	})

	// 注册消息处理器
	s.hub.RegisterHandler("chat", HandlerFunc(func(client *Client, message Message) error {
		metadata := client.GetMetadata()
		username := metadata["username"].(string)
		return s.handleChatMessage(client, message, username)
	}))

	s.hub.RegisterHandler("private", HandlerFunc(func(client *Client, message Message) error {
		metadata := client.GetMetadata()
		username := metadata["username"].(string)
		return s.handlePrivateMessage(client, message, username)
	}))

	s.hub.RegisterHandler("get_users", HandlerFunc(func(client *Client, message Message) error {
		return s.handleGetUsers(client)
	}))

	// 错误处理器
	s.hub.SetErrorHandler(func(client *Client, err error) {
		metadata := client.GetMetadata()
		username := metadata["username"]
		slog.Error("客户端发生错误", "username", username, "error", err)
	})
}

// handleChatMessage 处理聊天消息
func (s *ExampleChatServer) handleChatMessage(client *Client, message Message, username string) error {
	data := message.Data()

	content, ok := data["content"].(string)
	if !ok {
		return fmt.Errorf("消息内容不能为空")
	}

	// 广播聊天消息
	s.hub.Broadcast(NewMessage("chat", map[string]any{
		"username":  username,
		"content":   content,
		"timestamp": time.Now(),
	}))

	return nil
}

// handlePrivateMessage 处理私聊消息
func (s *ExampleChatServer) handlePrivateMessage(client *Client, message Message, username string) error {
	data := message.Data()

	target, ok := data["target"].(string)
	if !ok {
		return fmt.Errorf("目标用户不能为空")
	}

	content, ok := data["content"].(string)
	if !ok {
		return fmt.Errorf("消息内容不能为空")
	}

	// 发送私聊消息给目标用户
	privateMsg := NewMessage("private", map[string]any{
		"from":      username,
		"content":   content,
		"timestamp": time.Now(),
	})

	err := s.hub.SendToClient(context.Background(), target, privateMsg)
	if err != nil {
		// 通知发送者消息发送失败
		client.Send(context.Background(), NewErrorMessage(fmt.Sprintf("发送给 %s 失败: %v", target, err)))
		return err
	}

	// 通知发送者消息发送成功
	client.Send(context.Background(), NewMessage("private_sent", map[string]any{
		"target":    target,
		"content":   content,
		"timestamp": time.Now(),
	}))

	return nil
}

// handleGetUsers 处理获取在线用户列表
func (s *ExampleChatServer) handleGetUsers(client *Client) error {
	clients := s.hub.GetClients()
	users := make([]map[string]any, 0, len(clients))

	for _, c := range clients {
		metadata := c.GetMetadata()
		users = append(users, map[string]any{
			"username":   metadata["username"],
			"login_time": metadata["login_time"],
		})
	}

	client.Send(context.Background(), NewMessage("users_list", map[string]any{
		"users": users,
		"count": len(users),
	}))

	return nil
}

// ServeHTTP 处理 WebSocket 连接
func (s *ExampleChatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.hub.ServeHTTP(w, r)
}

// Close 关闭服务器
func (s *ExampleChatServer) Close() {
	s.hub.Close()
}

// ExampleUsage 使用示例
func ExampleUsage() {
	// 创建聊天服务器
	chatServer := NewExampleChatServer()
	defer chatServer.Close()

	// 设置路由
	http.HandleFunc("/ws", chatServer.ServeHTTP)

	// 提供静态文件服务（聊天室前端）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket 聊天室示例</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        #messages { border: 1px solid #ccc; height: 300px; overflow-y: scroll; padding: 10px; margin: 10px 0; }
        #messageInput { width: 70%; padding: 5px; }
        #sendBtn { padding: 5px 10px; }
        .message { margin: 5px 0; }
        .system { color: #666; font-style: italic; }
        .private { color: #007bff; }
        .error { color: #dc3545; }
    </style>
</head>
<body>
    <h1>WebSocket 聊天室示例</h1>

    <div>
        <label>Token: </label>
        <select id="tokenSelect">
            <option value="token123">Alice (token123)</option>
            <option value="token456">Bob (token456)</option>
            <option value="token789">Charlie (token789)</option>
        </select>
        <button id="connectBtn">连接</button>
        <button id="disconnectBtn" disabled>断开</button>
    </div>

    <div id="messages"></div>

    <div>
        <input type="text" id="messageInput" placeholder="输入消息..." disabled>
        <button id="sendBtn" disabled>发送</button>
        <button id="getUsersBtn" disabled>获取在线用户</button>
    </div>

    <div>
        <h3>私聊</h3>
        <input type="text" id="privateTarget" placeholder="目标用户名" disabled>
        <input type="text" id="privateMessage" placeholder="私聊内容" disabled>
        <button id="privateSendBtn" disabled>发送私聊</button>
    </div>

    <script>
        let ws = null;
        let connected = false;

        const messages = document.getElementById('messages');
        const messageInput = document.getElementById('messageInput');
        const sendBtn = document.getElementById('sendBtn');
        const connectBtn = document.getElementById('connectBtn');
        const disconnectBtn = document.getElementById('disconnectBtn');
        const getUsersBtn = document.getElementById('getUsersBtn');
        const tokenSelect = document.getElementById('tokenSelect');
        const privateTarget = document.getElementById('privateTarget');
        const privateMessage = document.getElementById('privateMessage');
        const privateSendBtn = document.getElementById('privateSendBtn');

        function addMessage(text, className = '') {
            const div = document.createElement('div');
            div.className = 'message ' + className;
            div.textContent = new Date().toLocaleTimeString() + ' - ' + text;
            messages.appendChild(div);
            messages.scrollTop = messages.scrollHeight;
        }

        function connect() {
            const token = tokenSelect.value;
            ws = new WebSocket('ws://localhost:8080/ws');

            ws.onopen = function() {
                // 发送鉴权消息
                ws.send(JSON.stringify({
                    type: 'auth',
                    data: { token: token }
                }));
            };

            ws.onmessage = function(event) {
                const msg = JSON.parse(event.data);

                switch(msg.type) {
                    case 'auth_ok':
                        connected = true;
                        updateUI();
                        addMessage('连接成功！', 'system');
                        break;
                    case 'error':
                        addMessage('错误: ' + msg.data.error, 'error');
                        break;
                    case 'chat':
                        addMessage(msg.data.username + ': ' + msg.data.content);
                        break;
                    case 'private':
                        addMessage('私聊来自 ' + msg.data.from + ': ' + msg.data.content, 'private');
                        break;
                    case 'private_sent':
                        addMessage('私聊已发送给 ' + msg.data.target + ': ' + msg.data.content, 'private');
                        break;
                    case 'user_online':
                        addMessage(msg.data.message, 'system');
                        break;
                    case 'user_offline':
                        addMessage(msg.data.message, 'system');
                        break;
                    case 'users_list':
                        const users = msg.data.users.map(u => u.username).join(', ');
                        addMessage('在线用户 (' + msg.data.count + '): ' + users, 'system');
                        break;
                }
            };

            ws.onclose = function() {
                connected = false;
                updateUI();
                addMessage('连接已断开', 'system');
            };

            ws.onerror = function(error) {
                addMessage('连接错误: ' + error, 'error');
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
            }
        }

        function sendMessage() {
            if (connected && messageInput.value.trim()) {
                ws.send(JSON.stringify({
                    type: 'chat',
                    data: { content: messageInput.value }
                }));
                messageInput.value = '';
            }
        }

        function sendPrivateMessage() {
            if (connected && privateTarget.value.trim() && privateMessage.value.trim()) {
                ws.send(JSON.stringify({
                    type: 'private',
                    data: {
                        target: privateTarget.value,
                        content: privateMessage.value
                    }
                }));
                privateMessage.value = '';
            }
        }

        function getUsers() {
            if (connected) {
                ws.send(JSON.stringify({
                    type: 'get_users',
                    data: {}
                }));
            }
        }

        function updateUI() {
            messageInput.disabled = !connected;
            sendBtn.disabled = !connected;
            getUsersBtn.disabled = !connected;
            privateTarget.disabled = !connected;
            privateMessage.disabled = !connected;
            privateSendBtn.disabled = !connected;
            connectBtn.disabled = connected;
            disconnectBtn.disabled = !connected;
            tokenSelect.disabled = connected;
        }

        // 事件监听
        connectBtn.onclick = connect;
        disconnectBtn.onclick = disconnect;
        sendBtn.onclick = sendMessage;
        getUsersBtn.onclick = getUsers;
        privateSendBtn.onclick = sendPrivateMessage;

        messageInput.onkeypress = function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        };

        privateMessage.onkeypress = function(e) {
            if (e.key === 'Enter') {
                sendPrivateMessage();
            }
        };

        // 定期发送心跳
        setInterval(function() {
            if (connected) {
                ws.send(JSON.stringify({
                    type: 'heartbeat',
                    data: {}
                }));
            }
        }, 30000);
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	log.Println("聊天服务器启动在 http://localhost:8080")
	log.Println("WebSocket 端点: ws://localhost:8080/ws")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 简单的 JWT 风格的 token 验证示例
func ExampleWithJWTAuth() {
	hub := NewHub()
	defer hub.Close()

	// 模拟 JWT 验证
	hub.SetAuthHandler(func(message Message) (string, error) {
		data := message.Data()
		token, ok := data["token"].(string)
		if !ok {
			return "", fmt.Errorf("token 不能为空")
		}

		// 这里应该是真正的 JWT 验证逻辑
		if !strings.HasPrefix(token, "Bearer ") {
			return "", fmt.Errorf("无效的 token 格式")
		}

		jwtToken := strings.TrimPrefix(token, "Bearer ")

		// 模拟解析 JWT
		switch jwtToken {
		case "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.user1":
			// client.SetMetadata("user_id", "1")
			// client.SetMetadata("username", "user1")
			// client.SetMetadata("role", "admin")
			return "user1", nil
		case "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.user2":
			// client.SetMetadata("user_id", "2")
			// client.SetMetadata("username", "user2")
			// client.SetMetadata("role", "user")
			return "user2", nil
		default:
			return "", fmt.Errorf("无效的 JWT token")
		}
	})

	// 注册消息处理器
	hub.RegisterHandler("broadcast", HandlerFunc(func(client *Client, message Message) error {
		metadata := client.GetMetadata()
		role := metadata["role"].(string)
		if role == "admin" {
			return handleAdminMessage(client, message, hub)
		}
		return fmt.Errorf("权限不足")
	}))

	hub.RegisterHandler("kick_user", HandlerFunc(func(client *Client, message Message) error {
		metadata := client.GetMetadata()
		role := metadata["role"].(string)
		if role == "admin" {
			return handleAdminMessage(client, message, hub)
		}
		return fmt.Errorf("权限不足")
	}))

	hub.RegisterHandler("chat", HandlerFunc(func(client *Client, message Message) error {
		return handleUserMessage(client, message, hub)
	}))

	http.HandleFunc("/ws", hub.ServeHTTP)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleAdminMessage(client *Client, message Message, hub Huber) error {
	switch message.Type() {
	case "broadcast":
		data := message.Data()
		content := data["content"].(string)
		hub.Broadcast(NewMessage("admin_broadcast", map[string]any{
			"content": content,
			"from":    "系统管理员",
		}))
	case "kick_user":
		data := message.Data()
		targetUser := data["user"].(string)
		// 这里可以实现踢出用户的逻辑
		hub.SendToClient(context.Background(), targetUser, NewErrorMessage("您已被管理员踢出"))
	default:
		return fmt.Errorf("未知的管理员命令: %s", message.Type())
	}
	return nil
}

func handleUserMessage(client *Client, message Message, hub Huber) error {
	switch message.Type() {
	case "chat":
		data := message.Data()
		content := data["content"].(string)
		metadata := client.GetMetadata()
		username := metadata["username"].(string)

		hub.Broadcast(NewMessage("chat", map[string]any{
			"username": username,
			"content":  content,
		}))
	default:
		return fmt.Errorf("普通用户不能执行此操作: %s", message.Type())
	}
	return nil
}
