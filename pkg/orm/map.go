package orm

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"maps"
)

type Map map[string]any

// UnmarshalJSON implements json.Unmarshaler.
func (i *Map) UnmarshalJSON(in []byte) error {
	// 由于设计用于 db 存储，限制大小
	if len(in) > 4096 {
		return errors.New("info is too large")
	}
	return json.Unmarshal(in, i)
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
