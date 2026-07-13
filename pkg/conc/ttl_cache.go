package conc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"
)

// ErrCacheNotFound 缓存未找到错误
var ErrCacheNotFound = errors.New("cache not found")

// Cacher 缓存接口
type Cacher interface {
	Set(context.Context, string, any)
	Del(context.Context, string)
	Get(context.Context, string, any) error
	SetNX(context.Context, string, any)
}

// TTLCache 不使用泛型，是考虑到使用 redis 时，也没办法用泛型
// 为了统一和代码简洁性，故而使用 any，而非泛型
type TTLCache struct {
	*TTLMap[string, any]
	ttl time.Duration
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{
		TTLMap: NewTTLMap[string, any](),
		ttl:    ttl,
	}
}

// Set 设置缓存值
func (t *TTLCache) Set(_ context.Context, key string, value any) {
	t.TTLMap.Store(key, value, t.ttl)
}

// Del 删除缓存值
func (t *TTLCache) Del(_ context.Context, key string) {
	t.TTLMap.Delete(key)
}

// Get 获取缓存值，将结果反序列化到 dest 中
func (t *TTLCache) Get(_ context.Context, key string, dest any) error {
	value, ok := t.TTLMap.Load(key)
	if !ok {
		return ErrCacheNotFound
	}
	// 使用反射进行高效的类型转换
	return assignByReflect(value, dest)
}

// assignByReflect 使用反射进行类型转换，比 JSON 序列化更高效
func assignByReflect(src any, dest any) error {
	srcVal := reflect.ValueOf(src)
	destVal := reflect.ValueOf(dest)

	if destVal.Kind() != reflect.Pointer {
		return errors.New("dest must be a pointer")
	}

	destElem := destVal.Elem()

	// 如果类型相同，直接赋值
	if srcVal.Type() == destElem.Type() {
		destElem.Set(srcVal)
		return nil
	}

	// 如果类型可转换，进行转换
	if srcVal.Type().ConvertibleTo(destElem.Type()) {
		destElem.Set(srcVal.Convert(destElem.Type()))
		return nil
	}

	// 如果反射转换失败，回退到 JSON 序列化（用于复杂类型转换）
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// SetNX 仅当键不存在时设置值
func (t *TTLCache) SetNX(_ context.Context, key string, value any) {
	// 检查键是否已存在（未过期）
	// 注意：不能使用 LoadOrStore，因为它会更新过期时间
	if _, exists := t.TTLMap.Load(key); !exists {
		t.TTLMap.Store(key, value, t.ttl)
	}
}
