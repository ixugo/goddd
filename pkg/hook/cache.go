package hook

import (
	"time"

	"github.com/ixugo/goddd/pkg/conc"
)

// CacheFn 缓存函数
type CacheFn[K comparable, V any] func(K) (V, bool, error)

// UseCache 使用内存缓存一些临时数据
// 由于是临时数据，其 value 应该是一次性数据，用完即丢
// 第二个返回参数用来标识是否命中缓存
func UseCache[K comparable, V any](fn func(K) (V, error)) CacheFn[K, V] {
	cache := make(map[K]V)
	return func(key K) (V, bool, error) {
		v, ok := cache[key]
		if ok {
			return v, true, nil
		}
		v, err := fn(key)
		if err == nil {
			cache[key] = v
		}
		return v, false, err
	}
}

// UseTTLCache 使用内存缓存一些临时数据
// 指定时间超时自动清理
func UseTTLCache[K comparable, V any](timeline time.Duration, fn func(K) (V, error)) CacheFn[K, V] {
	cache := conc.NewTTLMap[K, V]()
	return func(key K) (V, bool, error) {
		v, ok := cache.Load(key)
		if ok {
			return v, true, nil
		}
		v, err := fn(key)
		if err == nil {
			cache.Store(key, v, timeline)
		}
		return v, false, err
	}
}
