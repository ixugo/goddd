package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// 错误定义
var (
	ErrAuthFailed       = errors.New("鉴权失败")
	ErrAuthTimeout      = errors.New("鉴权超时，断开连接")
	ErrConnectionClosed = errors.New("连接已关闭")
	ErrMessageQueueFull = errors.New("消息队列已满")
	ErrInvalidMessage   = errors.New("无效消息格式")
	ErrClientNotFound   = errors.New("客户端不存在")
	ErrHubClosed        = errors.New("Hub 已关闭")
	ErrHandlerNotFound  = errors.New("处理器不存在")
)

// 系统消息类型
const (
	MsgTypeAuth      = "auth"
	MsgTypeError     = "error"
	MsgTypeClose     = "close"
	MsgTypeBroadcast = "broadcast"
	MsgTypeAuthOK    = "auth_ok"
)

// Client WebSocket 客户端实现
type Client struct {
	// id 存 string，鉴权后不再变，用原子值承载：ID() 是各收发路径的高频读操作，原子读避免锁开销
	id       atomic.Value
	conn     *websocket.Conn
	hub      *Hub
	send     chan Message
	metadata map[string]any
	mu       sync.RWMutex // 仅护 metadata 与 request
	ctx      context.Context
	cancel   context.CancelFunc
	// isAuth 每条入站消息都要检查，用原子布尔消除读路径上的互斥
	isAuth  atomic.Bool
	request *http.Request // 升级时的原始 HTTP 请求，供业务 handler 读取请求上下文
}

func newClient(conn *websocket.Conn, h *Hub, r *http.Request) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		conn:     conn,
		hub:      h,
		send:     make(chan Message, h.config.MessageQueueSize),
		metadata: make(map[string]any),
		ctx:      ctx,
		cancel:   cancel,
		request:  r,
	}
	// 连接建立即分配唯一 ID，鉴权处理器可返回业务 ID 覆盖之
	client.id.Store(uuid.NewString())
	return client
}

// Request 返回客户端升级时的原始 HTTP 请求，业务 handler 可用它拼接完整 URL 等。
// 测试或没有真实 HTTP 请求的场景可能返回 nil，调用方需自行判空。
func (c *Client) Request() *http.Request {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.request
}

// ID 返回客户端 ID：连接建立时为 UUID，鉴权成功后被业务 ID 覆盖（鉴权回调返回空 ID 则保留 UUID）
func (c *Client) ID() string {
	return c.id.Load().(string)
}

func (c *Client) setID(id string) {
	c.id.Store(id)
}

// Send 将消息投入发送队列，队列满时阻塞等待，直至队列可用、连接关闭或 ctx 超时。
func (c *Client) Send(ctx context.Context, message Message) error {
	select {
	case c.send <- message:
		return nil
	case <-c.ctx.Done():
		return ErrConnectionClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close 取消客户端上下文并关闭底层连接；合成客户端（无真实连接）仅取消上下文。
// 连接生命周期由库内部掌管（readPump/writePump/Hub），不对外暴露。
func (c *Client) close() error {
	c.cancel()
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// GetMetadata 返回客户端元数据的副本，修改返回值不影响内部状态
func (c *Client) GetMetadata() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]any)
	maps.Copy(result, c.metadata)
	return result
}

// SetMetadata 设置客户端级元数据，供业务 handler 存取连接上下文
func (c *Client) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// JoinGroup 将客户端加入指定分组（房间），重复加入幂等，分组不存在时隐式创建。
// 组名为空时直接忽略；连接断开后由 Hub 自动清出所有分组，无需手动退组。
func (c *Client) JoinGroup(groupID string) {
	if groupID == "" || c.hub == nil {
		return
	}
	// 成员变更不可丢弃（丢弃会导致分组计数失真），阻塞投递直至 run() 消费、
	// Hub 关闭或连接自身消亡（消亡后 leave 处理会兜底清理）
	select {
	case c.hub.groupOp <- groupOperation{client: c, groupID: groupID, join: true}:
	case <-c.hub.closeCh:
	case <-c.ctx.Done():
	}
}

// LeaveGroup 将客户端移出指定分组，不在分组中时为空操作。
func (c *Client) LeaveGroup(groupID string) {
	if groupID == "" || c.hub == nil {
		return
	}
	select {
	case c.hub.groupOp <- groupOperation{client: c, groupID: groupID, join: false}:
	case <-c.hub.closeCh:
	case <-c.ctx.Done():
	}
}

// touch 顺延读超时。任意入站帧（业务消息或 Pong）都视为活跃信号，统一走此入口。
func (c *Client) touch() {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.config.HeartbeatTimeout))
}

func (c *Client) setAuth(auth bool) {
	c.isAuth.Store(auth)
}

func (c *Client) isAuthenticated() bool {
	return c.isAuth.Load()
}

// IsAuthenticated 检查客户端是否已通过鉴权
func (c *Client) IsAuthenticated() bool {
	return c.isAuthenticated()
}

// 读取消息
func (c *Client) readPump() {
	// 鉴权/错误等业务回调在此协程内同步执行，其 panic 须拦截，避免全进程陪葬
	defer recoverLog("read pump panic", "client_id", c.ID())
	defer func() {
		if c.hub != nil {
			// 注销事件不可丢弃（丢弃会产生幽灵连接），阻塞投递直至 run() 消费或 Hub 关闭
			select {
			case c.hub.leave <- c:
			case <-c.hub.closeCh:
			}
		}
		c.close()
	}()

	c.conn.SetReadLimit(c.hub.config.MaxMessageSize)
	c.touch()
	c.conn.SetPongHandler(func(string) error {
		c.touch()
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, messageBytes, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("websocket error", "error", err)
				}
				return
			}

			// 任意入站消息均视为活跃信号，与 Pong 一样顺延读超时
			c.touch()

			// 解析消息信封：只解 type 与 data 外壳，data 子树保留原始字节，
			// 由强类型处理器直灌，不在此处物化 map
			var envelope struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(messageBytes, &envelope); err != nil {
				c.handleError(ErrInvalidMessage)
				continue
			}
			if envelope.Type == "" {
				c.handleError(ErrInvalidMessage)
				continue
			}

			// 创建消息对象
			message := &StandardMessage{
				MsgType: envelope.Type,
				raw:     envelope.Data,
			}
			if len(envelope.Data) == 0 {
				// 无 data 字段时，将除了 type 之外的所有字段作为 payload
				var rawMsg map[string]any
				if err := json.Unmarshal(messageBytes, &rawMsg); err == nil {
					delete(rawMsg, "type")
					if len(rawMsg) > 0 {
						message.Payload = rawMsg
					}
				}
			}

			// 处理系统消息
			switch envelope.Type {
			case MsgTypeAuth:
				if err := c.handleAuth(message); err != nil {
					continue
				}
			default:
				// 业务消息需要先鉴权
				if !c.isAuthenticated() {
					c.handleError(ErrAuthFailed)
					continue
				}
				// 使用注册的处理器处理消息
				if c.hub != nil {
					handler := c.hub.getHandler(envelope.Type)
					if err := c.safeHandle(handler, message); err != nil {
						c.handleError(err)
					}
				}
			}
		}
	}
}

// 写入消息
func (c *Client) writePump() {
	defer recoverLog("write pump panic", "client_id", c.ID())
	ticker := time.NewTicker(c.hub.config.HeartbeatInterval)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	// 等待鉴权
	authTimer := time.NewTimer(c.hub.config.AuthTimeout)
	defer authTimer.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-authTimer.C:
			if !c.isAuthenticated() {
				_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
				_ = c.conn.WriteJSON(NewMessage(MsgTypeError, ErrAuthTimeout.Error()))
				time.Sleep(time.Millisecond * 100)
				return
			}
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := message.Marshal()
			if err != nil {
				slog.Error("message to json error", "error", err)
				continue
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Error("write message error", "error", err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("write ping message error", "error", err)
				return
			}
		}
	}
}

func (c *Client) handleAuth(message Message) error {
	// 重复鉴权按幂等处理：直接回成功，不覆盖 ID、不重复登记
	if c.isAuthenticated() {
		return c.Send(c.ctx, NewMessage(MsgTypeAuthOK, nil))
	}

	if c.hub == nil || c.hub.authHandler == nil {
		c.setAuth(true)
		_ = c.Send(c.ctx, NewMessage(MsgTypeAuthOK, nil))
		// 鉴权成功后添加到 clientsByID
		if c.hub != nil {
			c.hub.addAuthenticatedClient(c.ctx, c)
		}
		return nil
	}

	clientID, err := c.hub.authHandler(message)
	if err != nil {
		_ = c.Send(c.ctx, NewErrorMessage(err.Error()))
		return err
	}

	// 更新客户端 ID；鉴权回调返回空 ID 时保留连接建立时分配的 UUID
	if clientID != "" {
		c.setID(clientID)
	}
	c.setAuth(true)

	// 鉴权成功后添加到 clientsByID
	if c.hub != nil {
		c.hub.addAuthenticatedClient(c.ctx, c)
	}

	c.Send(c.ctx, NewMessage(MsgTypeAuthOK, nil))
	return nil
}

// safeHandle 调用业务消息处理器，处理器 panic 时捕获并记录日志、返回 nil。
// 消息处理器由业务方编写，单条消息的 panic 不应打断读循环、更不应击垮进程。
func (c *Client) safeHandle(handler Handler, message Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("message handler panic", "client_id", c.ID(), "msg_type", message.Type(), "error", r, "stack", debug.Stack())
			err = nil
		}
	}()
	return handler.Handle(c, message)
}

func (c *Client) handleError(err error) {
	if c.hub != nil && c.hub.errorHandler != nil {
		c.hub.errorHandler(c, err)
	}
	c.Send(c.ctx, NewErrorMessage(err.Error()))
}
