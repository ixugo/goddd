package ws

import "encoding/json"

// Wrap 将强类型业务函数包装为 Handler：消息数据绑定到 T 后再调用 fn，
// 业务方直接操作具名字段，无需对 map 取值做类型断言。
// 字段类型不符时返回反序列化错误，由框架统一回错。
func Wrap[T any](fn func(client *Client, data T) error) Handler {
	return HandlerFunc(func(c *Client, m Message) error {
		var data T
		if err := bindMessage(m, &data); err != nil {
			return err
		}
		return fn(c, data)
	})
}

// bindMessage 将消息数据的原始字节直灌到 data。
// 消息无数据载荷时 T 保持零值，不视为错误。
func bindMessage[T any](m Message, data *T) error {
	raw := m.Data()
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, data)
}
