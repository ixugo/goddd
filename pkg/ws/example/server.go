package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ixugo/goddd/pkg/ws"
)

//go:embed websocket.html
var websocketHTML embed.FS

// WebSocket Hub
var hub ws.Huber

// 消息类型常量
const (
	typeProperty = "property"
	typeVersion  = "version"
	typeIPInfo   = "ip_info"
	typeReboot   = "reboot"
	typeUpgrade  = "upgrade"
	typeResponse = "response"
)

func main() {
	// 创建 WebSocket Hub
	hub = createHub()

	// WebSocket 服务
	http.Handle("/websocket", hub)

	// 内嵌的 HTML 页面服务
	http.Handle("/websocket.html", http.FileServer(http.FS(websocketHTML)))

	// 根路径重定向
	http.HandleFunc("/", handleRoot)

	// 启动服务器
	port := ":8080"
	log.Printf("服务器启动在端口 %s", port)
	log.Printf("访问 http://localhost%s 查看 WebSocket 演示页面", port)
	log.Printf("WebSocket 端点: ws://localhost%s/websocket", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}

// 鉴权处理器 - 处理框架级别的鉴权
func authHandler(message ws.Message) (clientID string, err error) {
	data := message.Data()

	token, ok := data["token"].(string)
	if !ok {
		return "", fmt.Errorf("token 不能为空")
	}

	// 简单的鉴权逻辑，实际项目中应该验证 token
	if token == "a67c2bacf5c691b6" {
		clientID := fmt.Sprintf("client_%d", time.Now().Unix())
		log.Printf("客户端鉴权成功，分配 ID: %s", clientID)
		return clientID, nil
	}
	log.Printf("客户端鉴权失败，无效的 token: %s", token)
	return "", fmt.Errorf("无效的鉴权令牌")
}

// 连接处理器
func connectHandler(client *ws.Client) error {
	log.Printf("新客户端连接: %s", client.ID())

	// 设置连接时间
	client.SetMetadata("connect_time", time.Now())

	// 发送欢迎消息
	welcomeMsg := ws.NewMessage("welcome", map[string]any{
		"message": "连接成功，欢迎使用 WebSocket 服务",
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	})

	return client.Send(context.Background(), welcomeMsg)
}

// 断开连接处理器
func disconnectHandler(client *ws.Client, err error) {
	log.Printf("客户端断开连接: %s, 原因: %v", client.ID(), err)
}

// 错误处理器
func errorHandler(client *ws.Client, err error) {
	log.Printf("客户端 %s 发生错误: %v", client.ID(), err)
}

// 根路径重定向处理
func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/websocket.html", http.StatusFound)
		return
	}
	http.NotFound(w, r)
}

// 创建并配置 Hub
func createHub() ws.Huber {
	h := ws.NewHub(func(c *ws.Config) {
		c.HeartbeatInterval = 30 * time.Second
		c.HeartbeatTimeout = 90 * time.Second
		c.AuthTimeout = 10 * time.Second
		c.MaxConnections = 100
	})

	// 设置处理器
	h.SetAuthHandler(authHandler)
	h.SetConnectHandler(connectHandler)
	h.SetDisconnectHandler(disconnectHandler)
	h.SetErrorHandler(errorHandler)

	// 注册消息处理器
	h.RegisterHandler(typeProperty, ws.HandlerFunc(func(client *ws.Client, message ws.Message) error {
		data := message.Data()
		cpu := data["cpu"].(float64)
		memory := data["memory"].(float64)
		disk := data["disk"].(float64)

		log.Printf("收到客户端 %s 的系统性能数据 - CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%",
			client.ID(), cpu, memory, disk)

		return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
			"status":  "success",
			"message": fmt.Sprintf("收到系统信息 - CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%", cpu, memory, disk),
		}))
	}))

	h.RegisterHandler(typeVersion, ws.HandlerFunc(func(client *ws.Client, message ws.Message) error {
		data := message.Data()
		version := data["version"].(string)
		businessVersion := data["business_version"].(string)

		log.Printf("收到客户端 %s 的版本信息 - 版本: %s, 业务版本: %s",
			client.ID(), version, businessVersion)

		client.SetMetadata("version", version)
		client.SetMetadata("business_version", businessVersion)

		return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
			"status":  "success",
			"message": fmt.Sprintf("收到版本信息 - 版本: %s, 业务版本: %s", version, businessVersion),
		}))
	}))

	h.RegisterHandler(typeIPInfo, ws.HandlerFunc(func(client *ws.Client, message ws.Message) error {
		data := message.Data()
		macAddress := data["mac_address"].(string)
		internalIP := data["internal_ip"].(string)
		internetIP := data["internet_ip"].(string)

		log.Printf("收到客户端 %s 的网络信息 - MAC: %s, 内网IP: %s, 外网IP: %s",
			client.ID(), macAddress, internalIP, internetIP)

		client.SetMetadata("mac_address", macAddress)
		client.SetMetadata("internal_ip", internalIP)
		client.SetMetadata("internet_ip", internetIP)

		return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
			"status":  "success",
			"message": fmt.Sprintf("网络信息 - MAC: %s, 内网IP: %s, 外网IP: %s", macAddress, internalIP, internetIP),
		}))
	}))

	h.RegisterHandler(typeReboot, ws.HandlerFunc(func(client *ws.Client, message ws.Message) error {
		log.Printf("收到客户端 %s 的重启指令", client.ID())
		return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
			"status":  "warning",
			"message": "收到重启指令，系统将在 10 秒后重启",
		}))
	}))

	h.RegisterHandler(typeUpgrade, ws.HandlerFunc(func(client *ws.Client, message ws.Message) error {
		log.Printf("收到客户端 %s 的升级指令", client.ID())
		return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
			"status":  "warning",
			"message": "收到更新指令，系统将开始更新流程",
		}))
	}))

	return h
}
