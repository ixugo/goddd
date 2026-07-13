package conc

import (
	"sync"
)

type Map[K comparable, V any] struct {
	data sync.Map
}

// NewMap 创建一个新的泛型 Map
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{}
}

// Store 存储键值对
func (m *Map[K, V]) Store(key K, value V) {
	m.data.Store(key, value)
}

// Load 根据键获取值
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.data.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	// 处理 nil 值的情况
	if v == nil {
		var zero V
		return zero, true
	}
	return v.(V), true
}

// LoadOrStore 获取或存储键值对
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	v, loaded := m.data.LoadOrStore(key, value)
	// 处理 nil 值的情况
	if v == nil {
		var zero V
		return zero, loaded
	}
	return v.(V), loaded
}

// LoadAndDelete 获取并删除键值对
func (m *Map[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.data.LoadAndDelete(key)
	if !loaded {
		var zero V
		return zero, false
	}
	// 处理 nil 值的情况
	if v == nil {
		var zero V
		return zero, true
	}
	return v.(V), true
}

// Delete 删除键值对
func (m *Map[K, V]) Delete(key K) {
	m.data.Delete(key)
}

// Range 遍历所有键值对
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.data.Range(func(key, value any) bool {
		// 处理 nil 值的情况
		var v V
		if value != nil {
			v = value.(V)
		}
		return f(key.(K), v)
	})
}

// Swap 交换键对应的值
func (m *Map[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	v, loaded := m.data.Swap(key, value)
	if !loaded {
		var zero V
		return zero, false
	}
	// 处理 nil 值的情况
	if v == nil {
		var zero V
		return zero, true
	}
	return v.(V), true
}

// CompareAndSwap 比较并交换值
func (m *Map[K, V]) CompareAndSwap(key K, old, ne V) bool {
	return m.data.CompareAndSwap(key, old, ne)
}

// CompareAndDelete 比较并删除键值对
func (m *Map[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.data.CompareAndDelete(key, old)
}

// Len 返回 map 中键值对的数量 (注意: 这是一个近似值)
func (m *Map[K, V]) Len() int {
	count := 0
	m.data.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// Keys 返回所有键的切片
func (m *Map[K, V]) Keys() []K {
	var keys []K
	m.data.Range(func(key, _ any) bool {
		keys = append(keys, key.(K))
		return true
	})
	return keys
}

// Values 返回所有值的切片
func (m *Map[K, V]) Values() []V {
	var values []V
	m.data.Range(func(_, value any) bool {
		// 处理 nil 值的情况
		var v V
		if value != nil {
			v = value.(V)
		}
		values = append(values, v)
		return true
	})
	return values
}

// Clear 清空所有键值对
func (m *Map[K, V]) Clear() {
	m.data.Range(func(key, _ any) bool {
		m.data.Delete(key)
		return true
	})
}
