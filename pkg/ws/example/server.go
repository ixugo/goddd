package main

import (
	"context"
	"embed"
	_ "embed"
	"encoding/json"
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

// propertyMsg 系统性能消息的数据结构
type propertyMsg struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
}

// versionMsg 版本信息消息的数据结构
type versionMsg struct {
	Version         string `json:"version"`
	BusinessVersion string `json:"business_version"`
}

// ipInfoMsg 网络信息消息的数据结构
type ipInfoMsg struct {
	MacAddress string `json:"mac_address"`
	InternalIP string `json:"internal_ip"`
	InternetIP string `json:"internet_ip"`
}

// handleProperty 处理系统性能上报，回执确认
func handleProperty(client *ws.Client, data propertyMsg) error {
	log.Printf("收到客户端 %s 的系统性能数据 - CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%",
		client.ID(), data.CPU, data.Memory, data.Disk)
	return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("收到系统信息 - CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%", data.CPU, data.Memory, data.Disk),
	}))
}

// handleVersion 处理版本信息上报，记录元数据后回执确认
func handleVersion(client *ws.Client, data versionMsg) error {
	log.Printf("收到客户端 %s 的版本信息 - 版本: %s, 业务版本: %s",
		client.ID(), data.Version, data.BusinessVersion)
	client.SetMetadata("version", data.Version)
	client.SetMetadata("business_version", data.BusinessVersion)
	return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("收到版本信息 - 版本: %s, 业务版本: %s", data.Version, data.BusinessVersion),
	}))
}

// handleIPInfo 处理网络信息上报，记录元数据后回执确认
func handleIPInfo(client *ws.Client, data ipInfoMsg) error {
	log.Printf("收到客户端 %s 的网络信息 - MAC: %s, 内网IP: %s, 外网IP: %s",
		client.ID(), data.MacAddress, data.InternalIP, data.InternetIP)
	client.SetMetadata("mac_address", data.MacAddress)
	client.SetMetadata("internal_ip", data.InternalIP)
	client.SetMetadata("internet_ip", data.InternetIP)
	return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("网络信息 - MAC: %s, 内网IP: %s, 外网IP: %s", data.MacAddress, data.InternalIP, data.InternetIP),
	}))
}

// handleReboot 处理重启指令，回执告警
func handleReboot(client *ws.Client, _ struct{}) error {
	log.Printf("收到客户端 %s 的重启指令", client.ID())
	return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
		"status":  "warning",
		"message": "收到重启指令，系统将在 10 秒后重启",
	}))
}

// handleUpgrade 处理升级指令，回执告警
func handleUpgrade(client *ws.Client, _ struct{}) error {
	log.Printf("收到客户端 %s 的升级指令", client.ID())
	return client.Send(context.Background(), ws.NewMessage(typeResponse, map[string]any{
		"status":  "warning",
		"message": "收到更新指令，系统将开始更新流程",
	}))
}

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
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(message.Data(), &data); err != nil {
		return "", err
	}
	token := data.Token
	if token == "" {
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

	// 注册消息处理器，ws.Wrap 将强类型函数包装为 Handler
	h.Handle(typeProperty, ws.Wrap(handleProperty))
	h.Handle(typeVersion, ws.Wrap(handleVersion))
	h.Handle(typeIPInfo, ws.Wrap(handleIPInfo))
	h.Handle(typeReboot, ws.Wrap(handleReboot))
	h.Handle(typeUpgrade, ws.Wrap(handleUpgrade))

	return h
}
