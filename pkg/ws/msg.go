package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// Message 消息接口。
// 实现必须是只读的：广播时同一实例被分发到所有连接并并发调用 Marshal，
// 携带可变状态的实现会产生数据竞争。
type Message interface {
	// Type 获取消息类型
	Type() string
	// Data 获取消息数据的原始 JSON 字节，无数据时返回 nil
	Data() json.RawMessage
	// Marshal 将消息序列化为 JSON 字节数组
	Marshal() ([]byte, error)
}

// StandardMessage 标准消息实现
type StandardMessage struct {
	MsgType string `json:"type"`
	Payload any    `json:"data,omitempty"`
	// raw 为入站消息 data 字段的原始字节，供强类型处理器直灌，省一次 map 往返
	raw json.RawMessage
	// cache 与 once 缓存序列化结果：广播时同一实例分发到所有连接，
	// 序列化只跑一趟，其余连接直取缓存
	cache []byte
	err   error
	once  sync.Once
	// Timestamp time.Time `json:"timestamp"`
	// ID        string    `json:"id"`
}

// ErrorMessage 错误消息实现
type ErrorMessage struct {
	MsgType string `json:"type"`
	Msg     string `json:"msg"`
}

// NewErrorMessage 创建新的错误消息
func NewErrorMessage(msg string) *ErrorMessage {
	return &ErrorMessage{
		MsgType: MsgTypeError,
		Msg:     msg,
	}
}

// Type 返回错误消息类型（恒为 MsgTypeError）
func (e *ErrorMessage) Type() string {
	return e.MsgType
}

// Data 返回 nil，错误消息无业务数据
func (e *ErrorMessage) Data() json.RawMessage {
	return nil
}

// Marshal 将错误消息序列化为 JSON 字节数组
func (e *ErrorMessage) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NewMessage 创建新的标准消息
func NewMessage(msgType string, data any) *StandardMessage {
	return &StandardMessage{
		MsgType: msgType,
		Payload: data,
	}
}

// Type 返回消息类型
func (m *StandardMessage) Type() string {
	return m.MsgType
}

// Data 返回消息数据的原始 JSON 字节。
// 入站消息直返所存原始字节；出站消息将 Payload 序列化为字节。
// 每次调用独立计算，不缓存、不改写自身状态，满足并发只读约束。
func (m *StandardMessage) Data() json.RawMessage {
	if len(m.raw) > 0 {
		return m.raw
	}
	if m.Payload == nil {
		return nil
	}
	raw, err := json.Marshal(m.Payload)
	if err != nil {
		slog.Error("message payload marshal error", "error", err)
		return nil
	}
	return raw
}

// Marshal 将消息序列化为 JSON 字节数组。
// 结果以 sync.Once 缓存：同一实例多次调用仅序列化一次，
// 广播分发场景下 N 个连接共享一趟序列化。
// 首次调用后 Payload 不可再改，否则缓存与实际内容脱节（消息合约本即只读）。
func (m *StandardMessage) Marshal() ([]byte, error) {
	m.once.Do(func() {
		m.cache, m.err = json.Marshal(m)
	})
	return m.cache, m.err
}

// UnmarshalJSON 实现 json.Unmarshaler，反序列化错误由基础库原样抛出。
// 内容被改写后序列化缓存随之失效，下次 Marshal 重新计算。
// 经别名类型绕行，避免 json.Unmarshal 递归调回本方法。
func (m *StandardMessage) UnmarshalJSON(data []byte) error {
	type alias StandardMessage
	if err := json.Unmarshal(data, (*alias)(m)); err != nil {
		return err
	}
	m.once = sync.Once{}
	m.cache, m.err = nil, nil
	return nil
}
