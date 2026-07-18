package ws

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ixugo/goddd/pkg/assert"
)

func TestExampleChatServerWS(t *testing.T) {
	s := NewExampleChatServer()
	defer s.Close()

	server := httptest.NewServer(s)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 无效 token 鉴权失败，收到错误消息
	bad, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer bad.Close()
	if !assert.NoError(t, bad.WriteJSON(map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "假token"},
	})) {
		return
	}
	msg := readUntilType(t, bad, MsgTypeError)
	assert.Equal(t, "无效的 token", msg["msg"])

	// 合法用户 Alice 上线
	alice, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer alice.Close()
	if !assert.NoError(t, alice.WriteJSON(map[string]any{
		"type": MsgTypeAuth,
		"data": map[string]any{"token": "token123"},
	})) {
		return
	}
	readUntilType(t, alice, MsgTypeAuthOK)

	// 查询在线用户列表
	if !assert.NoError(t, alice.WriteJSON(map[string]any{"type": "get_users"})) {
		return
	}
	msg = readUntilType(t, alice, "users_list")
	data := msg["data"].(map[string]any)
	assert.Equal(t, float64(1), data["count"])
}

// TestExampleServerDirectHandlers 直接调用示例服务器的三个业务处理方法，覆盖成功与参数缺失分支
func TestExampleServerDirectHandlers(t *testing.T) {
	s := NewExampleChatServer()
	defer s.Close()

	hub := s.hub.(*Hub)

	// Bob 以合成客户端身份登记为已认证在线用户，供私聊投递
	bob := newClient(nil, hub, nil)
	bob.setID("Bob")
	bob.setAuth(true)
	hub.join <- bob
	hub.addToID <- bob
	assert.Eventually(t, func() bool {
		return len(s.hub.GetClients()) == 1
	}, 2*time.Second, 20*time.Millisecond)

	alice := newClient(nil, hub, nil)
	alice.SetMetadata("username", "Alice")
	alice.SetMetadata("login_time", time.Now())

	// 群聊：正常广播 + 内容缺失报错
	if !assert.NoError(t, s.handleChatMessage(alice, NewMessage("chat", map[string]any{"content": "大家好"}), "Alice")) {
		return
	}
	assert.Error(t, s.handleChatMessage(alice, NewMessage("chat", map[string]any{}), "Alice"))

	// 私聊：成功投递 + 目标/内容缺失 + 目标不在线
	if !assert.NoError(t, s.handlePrivateMessage(alice, NewMessage("private", map[string]any{
		"target": "Bob", "content": "悄悄话",
	}), "Alice")) {
		return
	}
	assert.Equal(t, "private", drainUntilType(t, bob, "private").Type(), "Bob 应收到私聊消息")
	assert.Equal(t, "private_sent", drainUntilType(t, alice, "private_sent").Type(), "Alice 应收到送达回执")

	assert.Error(t, s.handlePrivateMessage(alice, NewMessage("private", map[string]any{}), "Alice"))
	assert.Error(t, s.handlePrivateMessage(alice, NewMessage("private", map[string]any{"target": "Bob"}), "Alice"))
	err := s.handlePrivateMessage(alice, NewMessage("private", map[string]any{
		"target": "不在线的人", "content": "hi",
	}), "Alice")
	assert.ErrorIs(t, err, ErrClientNotFound)
	assert.Equal(t, MsgTypeError, drainSend(t, alice).Type(), "投递失败应回传错误消息")

	// 在线用户列表：Bob 在线，数量为 1
	if !assert.NoError(t, s.handleGetUsers(alice)) {
		return
	}
	listMsg := drainSend(t, alice)
	assert.Equal(t, "users_list", listMsg.Type())

	// 经路由器拿到的处理器闭包也可直接驱动（metadata 已就位，不会触发类型断言 panic）
	if !assert.NoError(t, hub.getHandler("chat").Handle(alice, NewMessage("chat", map[string]any{"content": "经路由"}))) {
		return
	}
	if !assert.NoError(t, hub.getHandler("private").Handle(alice, NewMessage("private", map[string]any{
		"target": "Bob", "content": "经路由",
	}))) {
		return
	}
	if !assert.NoError(t, hub.getHandler("get_users").Handle(alice, NewMessage("get_users", nil))) {
		return
	}
	drainUntilType(t, alice, "private_sent")
	drainUntilType(t, alice, "users_list")
}

// badMessage 序列化必然失败的消息，用于触发写泵的 Marshal 错误分支

func TestExampleRoleHandlers(t *testing.T) {
	hub := NewHub()
	defer hub.Close()

	c := newClient(nil, hub, nil)
	c.SetMetadata("username", "root")

	// 管理员广播
	if !assert.NoError(t, handleAdminMessage(c, NewMessage("broadcast", map[string]any{"content": "系统通知"}), hub)) {
		return
	}
	// 踢人：目标不在线时 SendToClient 的错误被处理器吞掉，本身不报错
	if !assert.NoError(t, handleAdminMessage(c, NewMessage("kick_user", map[string]any{"user": "某人"}), hub)) {
		return
	}
	// 未知管理员命令报错
	assert.Error(t, handleAdminMessage(c, NewMessage("shutdown", nil), hub))

	// 普通用户聊天广播
	if !assert.NoError(t, handleUserMessage(c, NewMessage("chat", map[string]any{"content": "hi"}), hub)) {
		return
	}
	// 普通用户执行未知操作报错
	assert.Error(t, handleUserMessage(c, NewMessage("broadcast", nil), hub))
}

// TestMessageHandlerPanic 业务消息处理器 panic 时连接保持存活，后续消息照常处理
