package ws

import (
	"net/http"
	"time"
)

// Config 配置选项
type Config struct {
	// ReadBufferSize 底层连接的读缓冲区大小（字节），影响升级后每次系统调用的读取量
	ReadBufferSize int
	// WriteBufferSize 底层连接的写缓冲区大小（字节）
	WriteBufferSize int
	// MaxMessageSize 单条入站消息的最大字节数，超过则断开连接，防止大消息耗尽内存
	MaxMessageSize int64
	// WriteTimeout 单次写操作的超时时间，超时未写完视为连接异常
	WriteTimeout time.Duration
	// HeartbeatInterval 服务端向客户端发送 Ping 帧的间隔
	HeartbeatInterval time.Duration
	// HeartbeatTimeout 读超时时间，超过此时间未收到客户端任何数据（含 Pong）则判定连接死亡
	HeartbeatTimeout time.Duration
	// AuthTimeout 连接建立后等待鉴权的最长时间，超时未鉴权则主动断开
	AuthTimeout time.Duration
	// MaxConnections Hub 允许的最大并发连接数，超限的新连接会被拒绝
	MaxConnections int
	// MessageQueueSize 每个客户端发送队列的长度，队列满时发送方阻塞等待，直至 ctx 超时或连接关闭
	MessageQueueSize int
	// EventQueueSize Hub 内部事件通道（注册、注销、鉴权登记、广播）共用的缓冲长度
	EventQueueSize int
	// SendToClientQueueSize 定向发送请求通道的缓冲长度，写满后 SendToClient 阻塞等待直至 ctx 超时
	SendToClientQueueSize int
	// GetClientsQueueSize 获取客户端列表请求通道的缓冲长度，写满后 GetClients 阻塞等待直至 ctx 超时
	GetClientsQueueSize int
	// EnableCompression 是否启用 WebSocket 压缩扩展（permessage-deflate），会消耗少量 CPU
	EnableCompression bool
	// CheckOrigin 跨域校验函数，返回 false 则拒绝升级；默认放行所有来源
	CheckOrigin func(r *http.Request) bool
}

// DefaultConfig 返回默认的 WebSocket 配置
func DefaultConfig() *Config {
	return &Config{
		ReadBufferSize:        1024,             // 读取缓冲区大小
		WriteBufferSize:       1024,             // 写入缓冲区大小
		MaxMessageSize:        64 * 1024,        // 单条消息最大 64KB，覆盖常规 JSON 业务消息
		WriteTimeout:          10 * time.Second, // 单次写操作超时
		HeartbeatInterval:     30 * time.Second, // 心跳间隔
		HeartbeatTimeout:      90 * time.Second, // 心跳超时
		AuthTimeout:           15 * time.Second, // 连接后超过此时间，未鉴权，则断开连接
		MaxConnections:        10240,            // 最大连接数
		MessageQueueSize:      256,              // 消息队列长度
		EventQueueSize:        256,              // 内部事件通道缓冲
		SendToClientQueueSize: 16,               // 定向发送通道缓冲
		GetClientsQueueSize:   16,               // 客户端列表请求通道缓冲
		EnableCompression:     false,            // 启用压缩
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}
}
