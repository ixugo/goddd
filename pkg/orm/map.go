package orm

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"maps"
)

// StructMap 其目的是将已知的参数作为结构体调用，未知的参数不动
// 例如 s.Data.Username
type StructMap[T any] struct {
	Data T
	Map
}

// UnmarshalJSON implements [json.Unmarshaler].
func (s *StructMap[T]) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &s.Map); err != nil {
		return err
	}
	return json.Unmarshal(b, &s.Data)
}

// MarshalJSON implements [json.Marshaler].
func (s StructMap[T]) MarshalJSON() ([]byte, error) {
	if s.Map == nil {
		return json.Marshal(s.Data)
	}

	cache := make(map[string]any)
	{
		b, _ := json.Marshal(s.Data)
		_ = json.Unmarshal(b, &cache)
	}
	maps.Copy(s.Map, cache)
	return json.Marshal(s.Map)
}

func (i *StructMap[T]) Scan(input any) error {
	return JSONUnmarshal(input, i)
}

func (i StructMap[T]) Value() (driver.Value, error) {
	return json.Marshal(i)
}

// Map 因值是 any 类型，封装一些快速提取的函数
type Map map[string]any

// UnmarshalJSON implements json.Unmarshaler.
func (i *Map) UnmarshalJSON(in []byte) error {
	// 由于设计用于 db 存储，限制大小
	if len(in) > 4096 {
		return errors.New("info is too large")
	}
	// 先反序列化到普通 map，避免 *Map 实现了 Unmarshaler 导致递归调用
	m := make(map[string]any)
	if err := json.Unmarshal(in, &m); err != nil {
		return err
	}
	*i = m
	return nil
}

func (i *Map) Scan(input any) error {
	return JSONUnmarshal(input, i)
}

func (i Map) Value() (driver.Value, error) {
	return json.Marshal(i)
}

// Get 获取指定 key 的值
func (i Map) Get(key string) any {
	if i == nil {
		return nil
	}
	return i[key]
}

// GetString 获取指定 key 的字符串值
func (i Map) GetString(key string) string {
	if i == nil {
		return ""
	}
	v, ok := i[key].(string)
	if !ok {
		return ""
	}
	return v
}

// GetInt 获取指定 key 的整数值
func (i Map) GetInt(key string) int {
	if i == nil {
		return 0
	}
	switch v := i[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// GetBool 获取指定 key 的布尔值
func (i Map) GetBool(key string) bool {
	if i == nil {
		return false
	}
	v, ok := i[key].(bool)
	if !ok {
		return false
	}
	return v
}

// Set 设置指定 key 的值
func (i Map) Set(key string, value any) Map {
	if i == nil {
		i = make(Map)
	}
	i[key] = value
	return i
}

// Delete 删除指定 key
func (i Map) Delete(key string) Map {
	if i == nil {
		return i
	}
	delete(i, key)
	return i
}

// Has 判断是否存在指定 key
func (i Map) Has(key string) bool {
	if i == nil {
		return false
	}
	_, ok := i[key]
	return ok
}

// Merge 合并另一个 Map，相同 key 会被覆盖
func (i Map) Merge(other Map) Map {
	if i == nil {
		i = make(Map)
	}
	maps.Copy(i, other)
	return i
}
