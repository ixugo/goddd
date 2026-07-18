package ws

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

// Huber WebSocket 连接管理中心接口
type Huber interface {
	// ServeHTTP 处理 HTTP 升级为 WebSocket 的请求
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	// Broadcast 向所有已认证的客户端广播消息
	Broadcast(message Message)
	// SendToClient 向指定客户端发送消息，阻塞等待投递结果，ctx 控制最长等待时间
	SendToClient(ctx context.Context, clientID string, message Message) error
	// SendToClientAsync 向指定客户端发送消息，仅等待入队成功，不等待投递结果
	SendToClientAsync(ctx context.Context, clientID string, message Message) error
	// CloseClient 按 ID 踢下线：关闭该 ID 下所有连接（多标签页一并断开），阻塞等待处理完毕。
	// ID 不存在时目标态已达成，按幂等返回成功；断开走正常 leave 流程，触发 disconnect 回调
	CloseClient(clientID string) error
	// SendToGroup 向指定分组内所有客户端发送消息，阻塞等待投递完毕，ctx 控制最长等待时间。
	// 队列积压的慢连接跳过并记录告警日志，不剔除连接；分组不存在或为空时直接返回成功
	SendToGroup(ctx context.Context, groupID string, message Message) error
	// SendToGroupAsync 向指定分组发送消息，仅等待入队成功，不等待投递结果
	SendToGroupAsync(ctx context.Context, groupID string, message Message) error
	// GroupSize 获取指定分组内的连接数，分组不存在时返回 0
	GroupSize(groupID string) int
	// GetClients 获取所有已认证的客户端列表
	GetClients() []*Client
	// Close 关闭 Hub 并清理所有资源
	Close()
	// SetAuthHandler 设置鉴权处理器。
	// 处理器返回的 clientID 必须唯一标识业务主体：同一 ID 的多个连接视为同一主体（多标签页），
	// 不同主体共用一个 ID 会导致 SendToClient 串号投递。返回空 ID 时保留连接建立时分配的 UUID。
	SetAuthHandler(handler AuthHandler)
	// SetConnectHandler 设置客户端连接处理器
	SetConnectHandler(handler ConnectHandler)
	// SetDisconnectHandler 设置客户端断开连接处理器
	SetDisconnectHandler(handler DisconnectHandler)
	// SetErrorHandler 设置错误处理器
	SetErrorHandler(handler ErrorHandler)

	// Handle 注册指定类型的消息处理器，用法同 http.Handle
	Handle(msgType string, handler Handler)
	// SetDefaultHandler 设置默认消息处理器（处理未注册类型的消息）
	SetDefaultHandler(handler Handler)
}

// Config 配置选项定义于 config.go

// Handler 消息处理器接口
type Handler interface {
	Handle(client *Client, message Message) error
}

// HandlerFunc 函数类型实现 Handler 接口
type HandlerFunc func(client *Client, message Message) error

// Handle 实现 Handler 接口，直接调用函数本体
func (f HandlerFunc) Handle(client *Client, message Message) error {
	return f(client, message)
}

// MessageRouter 消息路由器，负责管理消息处理器
type MessageRouter struct {
	handlers       map[string]Handler
	defaultHandler Handler
	mu             sync.RWMutex
}

// NewMessageRouter 创建新的消息路由器实例
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		handlers: make(map[string]Handler),
		defaultHandler: HandlerFunc(func(client *Client, message Message) error {
			return client.Send(context.Background(), NewErrorMessage("未知的消息类型: "+message.Type()))
		}),
	}
}

// RegisterHandler 为指定消息类型注册处理器
func (r *MessageRouter) RegisterHandler(msgType string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[msgType] = handler
}

// SetDefaultHandler 设置默认消息处理器
func (r *MessageRouter) SetDefaultHandler(handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultHandler = handler
}

// GetHandler 获取指定消息类型的处理器，如果不存在则返回默认处理器
func (r *MessageRouter) GetHandler(msgType string) Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if handler, exists := r.handlers[msgType]; exists {
		return handler
	}
	return r.defaultHandler
}

// 回调函数类型定义
type (
	AuthHandler func(message Message) (clientID string, err error)

	ConnectHandler    func(client *Client) error
	DisconnectHandler func(client *Client, err error)
	ErrorHandler      func(client *Client, err error)
)

// sendToClientRequest 发送消息到指定客户端的请求
type sendToClientRequest struct {
	clientID string
	message  Message
	response chan error
}

// closeClientRequest 按 ID 踢下线的请求，response 仅作同步屏障：通知调用方连接已关闭
type closeClientRequest struct {
	clientID string
	response chan error
}

// getClientsRequest 获取客户端列表的请求
type getClientsRequest struct {
	response chan []*Client
}

// groupOperation 分组成员变更操作，join 为 true 表示入组，false 表示退组
type groupOperation struct {
	client  *Client
	groupID string
	join    bool
}

// sendToGroupRequest 发送消息到指定分组的请求。
// 组投递永不出错（慢连接跳过不算错），done 仅作同步屏障：通知调用方 run() 已投递完毕
type sendToGroupRequest struct {
	groupID string
	message Message
	done    chan struct{}
}

// groupSizeRequest 查询分组连接数的请求
type groupSizeRequest struct {
	groupID  string
	response chan int
}

// Hub WebSocket 连接管理中心实现
type Hub struct {
	clients      map[*Client]struct{}            // 以 *Client 为 key，使用 struct{} 节省内存
	clientsByID  map[string][]*Client            // 同一用户可能有多个 WS 连接（多标签页）
	groups       map[string]map[*Client]struct{} // 分组（房间）→ 成员集合
	clientGroups map[*Client]map[string]struct{} // 连接 → 所属分组，供断开时反向清理
	join         chan *Client
	leave        chan *Client
	addToID      chan *Client // 鉴权成功后添加到 clientsByID

	broadcast    chan Message
	sendToClient chan sendToClientRequest
	closeClient  chan closeClientRequest
	getClients   chan getClientsRequest
	groupOp      chan groupOperation
	sendToGroup  chan sendToGroupRequest
	groupSize    chan groupSizeRequest
	closeCh      chan struct{}
	config       *Config
	upgrader     websocket.Upgrader
	closed       int32 // 使用原子操作

	// 消息路由器
	router *MessageRouter

	// 回调处理器
	authHandler       AuthHandler
	connectHandler    ConnectHandler
	disconnectHandler DisconnectHandler
	errorHandler      ErrorHandler
}

// ConfigOption 配置覆写函数，经 NewHub 传入以修改默认配置
type ConfigOption func(*Config)

// NewHub 创建新的 WebSocket Hub 实例
func NewHub(opt ...ConfigOption) *Hub {
	config := DefaultConfig()
	for _, o := range opt {
		o(config)
	}

	h := Hub{
		clients:      make(map[*Client]struct{}),
		clientsByID:  make(map[string][]*Client),
		groups:       make(map[string]map[*Client]struct{}),
		clientGroups: make(map[*Client]map[string]struct{}),
		join:         make(chan *Client, config.EventQueueSize),
		leave:        make(chan *Client, config.EventQueueSize),
		addToID:      make(chan *Client, config.EventQueueSize),
		broadcast:    make(chan Message, config.EventQueueSize),
		sendToClient: make(chan sendToClientRequest, config.SendToClientQueueSize),
		closeClient:  make(chan closeClientRequest, config.EventQueueSize),
		getClients:   make(chan getClientsRequest, config.GetClientsQueueSize),
		groupOp:      make(chan groupOperation, config.EventQueueSize),
		sendToGroup:  make(chan sendToGroupRequest, config.SendToClientQueueSize),
		groupSize:    make(chan groupSizeRequest, config.GetClientsQueueSize),
		closeCh:      make(chan struct{}),
		config:       config,
		router:       NewMessageRouter(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			EnableCompression: config.EnableCompression,
			CheckOrigin:       config.CheckOrigin,
		},
	}

	go h.run()
	return &h
}

func (h *Hub) isClosed() bool {
	return atomic.LoadInt32(&h.closed) == 1
}

func (h *Hub) run() {
	// 事件循环是 Hub 的心脏，意外 panic 须拦截在此，避免全进程陪葬
	defer recoverLog("hub run loop panic")
	for {
		select {
		case client := <-h.join:
			h.joinClient(client)
		case client := <-h.leave:
			h.leaveClient(client)
		case client := <-h.addToID:
			h.addClientToID(client)
		case message := <-h.broadcast:
			h.broadcastToAll(message)
		case req := <-h.sendToClient:
			h.sendTo(req)
		case req := <-h.closeClient:
			h.closeClientByID(req)
		case op := <-h.groupOp:
			h.handleGroupOp(op)
		case req := <-h.sendToGroup:
			h.sendToGroupMembers(req)
		case req := <-h.groupSize:
			req.response <- len(h.groups[req.groupID])
		case req := <-h.getClients:
			clients := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				if client != nil && client.IsAuthenticated() {
					clients = append(clients, client)
				}
			}
			req.response <- clients
		case <-h.closeCh:
			h.closeChannel()
			return
		}
	}
}

func (h *Hub) closeChannel() {
	atomic.StoreInt32(&h.closed, 1)

	// 关闭所有客户端连接
	for client := range h.clients {
		if client != nil {
			if err := client.close(); err != nil {
				slog.Error("close client error", "client_id", client.ID(), "err", err)
			}
		}
	}

	// 注意：内部通道不作 close。
	// 发送方遍布各 goroutine（readPump、业务 handler、HTTP 接入层），
	// close 通道会与在途发送形成 send-on-closed-channel 竞态 panic（TOCTOU）。
	// 通道随 Hub 对象被 GC 回收即可，发送方一律以 closeCh 作为逃逸路径。
}

func (h *Hub) joinClient(client *Client) {
	if client == nil {
		return
	}
	// 连接在 join 事件排队期间已断开（leave 先于 join 被处理的重排窗口），
	// 登记即产生永不移除的幽灵连接，直接放弃
	if client.ctx.Err() != nil {
		return
	}
	if len(h.clients) >= h.config.MaxConnections {
		// run() 内不可阻塞等待，队列满则直接丢弃该提示
		select {
		case client.send <- NewErrorMessage("连接数已达上限"):
		default:
		}
		if err := client.close(); err != nil {
			slog.Error("close client error", "client_id", client.ID(), "err", err)
		}
		return
	}
	h.clients[client] = struct{}{}
	if h.connectHandler != nil {
		// 必须在独立协程中执行连接回调。
		// 回调内部可能调用 SendToClient 等需要 run() 协程消费的方法，
		// 同步执行会导致 run() 等待自身而死锁。
		safeGo(func() { _ = h.connectHandler(client) })
	}
}

func (h *Hub) addClientToID(client *Client) {
	if client == nil {
		return
	}
	if client.IsAuthenticated() {
		id := client.ID()
		h.clientsByID[id] = append(h.clientsByID[id], client)
	}
}

func (h *Hub) leaveClient(client *Client) {
	if client == nil {
		return
	}
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		h.removeFromIDMap(client)
		// 断开连接自动清出所有分组，业务方无需在断开回调中手动清理
		for groupID := range h.clientGroups[client] {
			h.removeFromGroup(client, groupID)
		}
		delete(h.clientGroups, client)
		// client.send 不作 close：发送方遍布多个 goroutine，
		// close 会与在途发送形成 send-on-closed-channel 竞态 panic。
		// writePump 由 ctx 取消驱动退出，通道随 Client 被 GC 回收。
	}
	// 必须在独立协程中执行 disconnect 回调。
	// 回调内部可能调用 GetClients() / SendToClient() 等需要与 run() 协程通信的方法，
	// 如果直接在 run() 中调用会导致死锁（自己等自己）。
	if h.disconnectHandler != nil {
		safeGo(func() { h.disconnectHandler(client, nil) })
	}
}

func (h *Hub) broadcastToAll(message Message) {
	if message == nil {
		return
	}
	// 发送队列满的慢连接跳过即可：连接的删除只能由 run() 处理 leave 事件执行，
	// 广播路径不得删除或关闭客户端，仅记录跳过名单
	var skipped []string
	for client := range h.clients {
		if client != nil && client.IsAuthenticated() {
			select {
			case client.send <- message:
			default:
				skipped = append(skipped, client.ID())
			}
		}
	}
	if len(skipped) > 0 {
		slog.Warn("broadcast skipped slow clients", "count", len(skipped), "client_ids", skipped)
	}
}

// handleGroupOp 处理分组成员变更，仅在 run() 内执行，正反向索引同步维护
func (h *Hub) handleGroupOp(op groupOperation) {
	if op.client == nil || op.groupID == "" {
		return
	}
	if !op.join {
		h.removeFromGroup(op.client, op.groupID)
		return
	}
	if h.groups[op.groupID] == nil {
		h.groups[op.groupID] = make(map[*Client]struct{})
	}
	h.groups[op.groupID][op.client] = struct{}{}
	if h.clientGroups[op.client] == nil {
		h.clientGroups[op.client] = make(map[string]struct{})
	}
	h.clientGroups[op.client][op.groupID] = struct{}{}
}

// removeFromGroup 将客户端移出指定分组；不在分组中为空操作，空分组即时销毁
func (h *Hub) removeFromGroup(client *Client, groupID string) {
	members := h.groups[groupID]
	if _, ok := members[client]; !ok {
		return
	}
	delete(members, client)
	if len(members) == 0 {
		delete(h.groups, groupID)
	}
	delete(h.clientGroups[client], groupID)
	if len(h.clientGroups[client]) == 0 {
		delete(h.clientGroups, client)
	}
}

// sendToGroupMembers 向分组内所有成员投递消息。
// 发送队列满的慢连接跳过即可：连接的删除只能由 run() 处理 leave 事件执行，
// 分组投递路径不得删除或关闭客户端，仅记录跳过名单
func (h *Hub) sendToGroupMembers(req sendToGroupRequest) {
	var skipped []string
	for client := range h.groups[req.groupID] {
		if client == nil {
			continue
		}
		select {
		case client.send <- req.message:
		default:
			skipped = append(skipped, client.ID())
		}
	}
	if len(skipped) > 0 {
		slog.Warn("group broadcast skipped slow clients", "group", req.groupID, "count", len(skipped), "client_ids", skipped)
	}
	close(req.done)
}

// removeFromIDMap 从 clientsByID 中移除指定 client
func (h *Hub) removeFromIDMap(client *Client) {
	id := client.ID()
	conns := h.clientsByID[id]
	filtered := conns[:0]
	for _, c := range conns {
		if c != client {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		delete(h.clientsByID, id)
	} else {
		h.clientsByID[id] = filtered
	}
}

// closeClientByID 关闭指定 ID 下的所有连接。
// close() 取消 ctx 并关闭底层连接，readPump 退出后走 leave 事件完成除名与退组，
// 此处不直接改动 clientsByID，避免与 leave 处理产生双重清理
func (h *Hub) closeClientByID(req closeClientRequest) {
	for _, client := range h.clientsByID[req.clientID] {
		_ = client.close()
	}
	req.response <- nil
}

func (h *Hub) sendTo(req sendToClientRequest) {
	conns := h.clientsByID[req.clientID]
	if len(conns) == 0 {
		req.response <- ErrClientNotFound
		return
	}
	var lastErr error
	for _, client := range conns {
		if client.IsAuthenticated() {
			// run() 内不可阻塞等待客户端队列，投递失败即记录错误
			select {
			case client.send <- req.message:
			case <-client.ctx.Done():
				lastErr = ErrConnectionClosed
			default:
				lastErr = ErrMessageQueueFull
			}
		}
	}
	req.response <- lastErr
}

// addAuthenticatedClient 将已鉴权的客户端添加到 clientsByID 映射中。
// 以 closeCh 与 ctx 为逃逸路径：Hub 关闭或调用方超时即放弃登记，不阻塞也不触碰已关闭通道。
func (h *Hub) addAuthenticatedClient(ctx context.Context, client *Client) {
	select {
	case h.addToID <- client:
	case <-h.closeCh:
	case <-ctx.Done():
	}
}

// ServeHTTP 实现 http.Handler，处理 WebSocket 升级并将新连接注册到 Hub
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isClosed() {
		http.Error(w, "Hub is closed", http.StatusServiceUnavailable)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error", "error", err)
		return
	}

	client := newClient(conn, h, r)

	select {
	case h.join <- client:
	case <-h.closeCh:
		// 升级完成后 Hub 恰被关闭，直接断开连接，避免 HTTP 协程永久挂起
		_ = client.close()
		return
	case <-r.Context().Done():
		// HTTP 请求方已取消（客户端断开等），同样断开避免挂起
		_ = client.close()
		return
	}
	go client.writePump()
	go client.readPump()
}

// Broadcast 向所有已认证的客户端广播消息；广播通道满时丢弃并记录告警日志
func (h *Hub) Broadcast(message Message) {
	if h.isClosed() {
		return
	}

	select {
	case h.broadcast <- message:
	default:
		slog.Warn("broadcast channel is full")
	}
}

// SendToClient 同步向指定客户端发送消息，阻塞等待 run() 处理完毕并返回投递结果。
// ctx 控制入队与等待响应的最长时间；Hub 关闭时立即返回 ErrHubClosed。
func (h *Hub) SendToClient(ctx context.Context, clientID string, message Message) error {
	if h.isClosed() {
		return ErrHubClosed
	}

	req := sendToClientRequest{
		clientID: clientID,
		message:  message,
		response: make(chan error, 1),
	}

	select {
	case h.sendToClient <- req:
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.response:
		return err
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SendToClientAsync 异步向指定客户端发送消息：等待请求入队即返回，不等待投递结果。
// 返回值仅表示入队是否成功（Hub 关闭或 ctx 超时），不代表消息是否送达。
func (h *Hub) SendToClientAsync(ctx context.Context, clientID string, message Message) error {
	if h.isClosed() {
		return ErrHubClosed
	}

	req := sendToClientRequest{
		clientID: clientID,
		message:  message,
		// 缓冲为 1，run() 写入结果后无人读取也不会阻塞
		response: make(chan error, 1),
	}

	select {
	case h.sendToClient <- req:
		return nil
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CloseClient 按 ID 踢下线：关闭该 ID 下所有连接（多标签页一并断开），阻塞等待 run() 处理完毕。
// ID 不存在时目标态已达成，按幂等返回 nil；Hub 关闭时返回 ErrHubClosed。
func (h *Hub) CloseClient(clientID string) error {
	if h.isClosed() {
		return ErrHubClosed
	}

	req := closeClientRequest{
		clientID: clientID,
		response: make(chan error, 1),
	}

	select {
	case h.closeClient <- req:
	case <-h.closeCh:
		return ErrHubClosed
	}
	select {
	case err := <-req.response:
		return err
	case <-h.closeCh:
		return ErrHubClosed
	}
}

// SendToGroup 同步向指定分组内所有客户端发送消息，阻塞等待 run() 投递完毕。
// 队列积压的慢连接跳过并记录告警日志；ctx 控制入队与等待响应的最长时间；
// Hub 关闭时立即返回 ErrHubClosed。
func (h *Hub) SendToGroup(ctx context.Context, groupID string, message Message) error {
	if h.isClosed() {
		return ErrHubClosed
	}

	req := sendToGroupRequest{
		groupID: groupID,
		message: message,
		done:    make(chan struct{}),
	}

	select {
	case h.sendToGroup <- req:
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-req.done:
		return nil
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SendToGroupAsync 异步向指定分组发送消息：等待请求入队即返回，不等待投递结果。
// 返回值仅表示入队是否成功（Hub 关闭或 ctx 超时），不代表消息是否送达。
func (h *Hub) SendToGroupAsync(ctx context.Context, groupID string, message Message) error {
	if h.isClosed() {
		return ErrHubClosed
	}

	req := sendToGroupRequest{
		groupID: groupID,
		message: message,
		// run() 以 close 通知投递完毕，无人接收亦无碍
		done: make(chan struct{}),
	}

	select {
	case h.sendToGroup <- req:
		return nil
	case <-h.closeCh:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GroupSize 获取指定分组内的连接数；分组不存在返回 0，Hub 关闭返回 0。
func (h *Hub) GroupSize(groupID string) int {
	if h.isClosed() {
		return 0
	}

	req := groupSizeRequest{
		groupID:  groupID,
		response: make(chan int, 1),
	}

	select {
	case h.groupSize <- req:
	case <-h.closeCh:
		return 0
	}
	select {
	case size := <-req.response:
		return size
	case <-h.closeCh:
		return 0
	}
}

// GetClients 返回所有已认证客户端的列表快照；Hub 关闭时返回空列表
func (h *Hub) GetClients() []*Client {
	if h.isClosed() {
		return []*Client{}
	}

	req := getClientsRequest{
		response: make(chan []*Client, 1),
	}

	select {
	case h.getClients <- req:
	case <-h.closeCh:
		return []*Client{}
	}
	select {
	case clients := <-req.response:
		return clients
	case <-h.closeCh:
		return []*Client{}
	}
}

// Close 关闭 Hub 并断开所有客户端连接，幂等
func (h *Hub) Close() {
	if atomic.CompareAndSwapInt32(&h.closed, 0, 1) {
		close(h.closeCh)
	}
}

// SetAuthHandler type=auth
func (h *Hub) SetAuthHandler(handler AuthHandler) {
	h.authHandler = handler
}

// SetConnectHandler 设置客户端连接处理器，新连接注册后于独立 goroutine 触发；
// 触发时连接尚未鉴权，ID 为 UUID
func (h *Hub) SetConnectHandler(handler ConnectHandler) {
	h.connectHandler = handler
}

// SetDisconnectHandler 设置客户端断开连接处理器，连接除名后于独立 goroutine 触发
func (h *Hub) SetDisconnectHandler(handler DisconnectHandler) {
	h.disconnectHandler = handler
}

// SetErrorHandler 设置错误处理器，消息解析失败、处理器报错、未鉴权发业务消息时触发
func (h *Hub) SetErrorHandler(handler ErrorHandler) {
	h.errorHandler = handler
}

// Handle 注册指定类型的业务消息处理器
func (h *Hub) Handle(msgType string, handler Handler) {
	h.router.RegisterHandler(msgType, handler)
}

// SetDefaultHandler 设置默认消息处理器，处理未注册类型的消息
func (h *Hub) SetDefaultHandler(handler Handler) {
	h.router.SetDefaultHandler(handler)
}

// getHandler 获取指定类型的处理器
func (h *Hub) getHandler(msgType string) Handler {
	return h.router.GetHandler(msgType)
}
