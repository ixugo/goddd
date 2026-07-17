package ws

import "encoding/json"

// Message 消息接口。
// 实现必须是只读的：广播时同一实例被分发到所有连接并并发调用 Marshal，
// 携带可变状态的实现会产生数据竞争。
type Message interface {
	// Type 获取消息类型
	Type() string
	// Data 获取消息数据
	Data() map[string]any
	// Marshal 将消息序列化为 JSON 字节数组
	Marshal() ([]byte, error)
	// Unmarshal 从数据中反序列化消息
	Unmarshal(data any) error
}

// StandardMessage 标准消息实现
type StandardMessage struct {
	MsgType string `json:"type"`
	Payload any    `json:"data,omitempty"`
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

// Data 返回空 map，错误消息无业务数据
func (e *ErrorMessage) Data() map[string]any {
	return make(map[string]any)
}

// Marshal 将错误消息序列化为 JSON 字节数组
func (e *ErrorMessage) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal 从 JSON 字节数组或字符串反序列化错误消息
func (e *ErrorMessage) Unmarshal(data any) error {
	if bytes, ok := data.([]byte); ok {
		return json.Unmarshal(bytes, e)
	}
	if str, ok := data.(string); ok {
		return json.Unmarshal([]byte(str), e)
	}
	return json.Unmarshal([]byte{}, e) // 尝试从空数据反序列化
}

// NewMessage 创建新的标准消息
func NewMessage(msgType string, data any) *StandardMessage {
	return &StandardMessage{
		MsgType: msgType,
		Payload: data,
		// Timestamp: time.Now(),
		// ID:        uuid.New().String(),
	}
}

// Type 返回消息类型
func (m *StandardMessage) Type() string {
	return m.MsgType
}

// Data 以 map 形式返回消息数据；Payload 非 map[string]any 类型时返回空 map
func (m *StandardMessage) Data() map[string]any {
	if data, ok := m.Payload.(map[string]any); ok {
		return data
	}
	// 如果 Payload 不是 map[string]any 类型，返回空 map
	return make(map[string]any)
}

// Marshal 将消息序列化为 JSON 字节数组
func (m *StandardMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal 从 JSON 字节数组或字符串反序列化消息
func (m *StandardMessage) Unmarshal(data any) error {
	if bytes, ok := data.([]byte); ok {
		return json.Unmarshal(bytes, m)
	}
	if str, ok := data.(string); ok {
		return json.Unmarshal([]byte(str), m)
	}
	return json.Unmarshal([]byte{}, m) // 尝试从空数据反序列化
}
