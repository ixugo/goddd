package conc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/assert"
)

// closed 返回一个轮询函数，供 assert.Eventually 检查 channel 是否已关闭。
// 把 select 等待异步结果的模式统一成轮询，避免固定长 sleep 拖慢测试。
func closed(ch <-chan struct{}) func() bool {
	return func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
}

// ==== conc.go ====

// spyTracer 是记录 Error 调用次数的 Tracer，用于断言 panic 被 recover 后产生了错误日志。
// Error 会在多个 goroutine 里被调用，计数必须用 atomic 保证并发安全。
type spyTracer struct {
	count atomic.Int32
}

func (s *spyTracer) Error(string, ...any) {
	s.count.Add(1)
}

// TestG 验证 GoRun 的 panic 兜底：
// 两个任务各自 panic，Wait 正常返回，且每次 panic 都通过 Tracer 记录一条错误日志
func TestG(t *testing.T) {
	spy := &spyTracer{}
	g := New(spy)
	g.GoRun(func() {
		panic("test")
	})
	g.GoRun(func() {
		panic("test1")
	})
	g.Wait()
	assert.Equal(t, int32(2), spy.count.Load(), "两次 panic 应各产生一条错误日志")
}

// TestUnsafeWaitWithContext 验证超时路径：
// 任务跑不完，ctx 超时后应返回 context.DeadlineExceeded
func TestUnsafeWaitWithContext(t *testing.T) {
	g := New(nil)
	g.GoRun(func() {
		time.Sleep(10 * time.Second)
	})
	g.GoRun(func() {
		time.Sleep(10 * time.Second)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := g.UnsafeWaitWithContext(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestGoSafeNormal 验证 GoSafe 正常执行路径：
// 用 channel 确认函数确实在 goroutine 里跑完了
func TestGoSafeNormal(t *testing.T) {
	done := make(chan struct{})
	GoSafe(func() {
		close(done)
	})
	assert.Eventually(t, closed(done), time.Second, 10*time.Millisecond, "GoSafe 的函数未在超时内执行")
}

// TestGoSafePanic 验证 GoSafe 的 panic 兜底路径：
// panic 不应炸掉测试进程，能被内部 recover 掉
func TestGoSafePanic(t *testing.T) {
	done := make(chan struct{})
	GoSafe(func() {
		defer close(done)
		panic("boom")
	})
	assert.Eventually(t, closed(done), time.Second, 10*time.Millisecond, "GoSafe 的函数未在超时内执行")
}

// TestTimer 验证 Timer 轮询语义：
// first 短、every 短，fn 至少被调用 2 次后取消 ctx，Timer 应正常退出
func TestTimer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var count atomic.Int32
	done := make(chan struct{})
	go func() {
		Timer(ctx, 10*time.Millisecond, 10*time.Millisecond, func() {
			if count.Add(1) >= 2 {
				cancel()
			}
		})
		close(done)
	}()
	assert.Eventually(t, closed(done), 2*time.Second, 10*time.Millisecond, "Timer 未在 ctx 取消后退出")
	assert.True(t, count.Load() >= 2, "expect fn 至少调用 2 次, got %d", count.Load())
}

// TestDefaultTimer 验证 DefaultTimer 的取消路径：
// 首次执行在 3 秒后，测试中直接取消 ctx，函数应立即返回且不调用 fn
func TestDefaultTimer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var called atomic.Bool
	done := make(chan struct{})
	go func() {
		DefaultTimer(ctx, time.Second, func() {
			called.Store(true)
		})
		close(done)
	}()
	assert.Eventually(t, closed(done), time.Second, 10*time.Millisecond, "DefaultTimer 未在 ctx 取消后退出")
	assert.False(t, called.Load(), "ctx 已取消, fn 不应被调用")
}

// TestUnsafeWaitWithContextDone 验证 UnsafeWaitWithContext 的正常完成路径：
// 任务快速跑完，应在 ctx 超时前返回 nil
func TestUnsafeWaitWithContextDone(t *testing.T) {
	g := New(nil)
	g.GoRun(func() {})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, g.UnsafeWaitWithContext(ctx))
}

// TestDefaultTracer 验证 DefaultTracer 把 msg 和 args 原样透传到 slog
func TestDefaultTracer(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	DefaultTracer{}.Error("test message", "k", "v")
	out := buf.String()
	assert.Contains(t, out, "test message")
	assert.Contains(t, out, "k=v")
}

// ==== sync_map.go ====

// TestMapStoreLoad 验证 Store/Load 的基本读写与零值边界：
// 存在的 key 读回值，不存在的 key 返回零值 + false
func TestMapStoreLoad(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	v, ok := m.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
	v, ok = m.Load("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestMapNilValue 验证存储 nil 值的分支：
// V 为 any 时存 nil，Load 应返回零值但 ok 为 true
func TestMapNilValue(t *testing.T) {
	m := NewMap[string, any]()
	m.Store("k", nil)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Nil(t, v)
}

// TestMapLoadOrStore 验证 LoadOrStore 两种路径：
// 首次返回刚存的值 loaded=false，再次调用返回已存在的值 loaded=true 且不覆盖
func TestMapLoadOrStore(t *testing.T) {
	m := NewMap[string, int]()
	v, loaded := m.LoadOrStore("a", 1)
	assert.False(t, loaded)
	assert.Equal(t, 1, v)
	v, loaded = m.LoadOrStore("a", 2)
	assert.True(t, loaded)
	assert.Equal(t, 1, v)
}

// TestMapLoadAndDelete 验证 LoadAndDelete：
// 命中时返回值并删除，未命中时返回零值 + false
func TestMapLoadAndDelete(t *testing.T) {
	m := NewMap[string, int]()
	_, loaded := m.LoadAndDelete("a")
	assert.False(t, loaded, "expect loaded=false for missing key")
	m.Store("a", 1)
	v, loaded := m.LoadAndDelete("a")
	assert.True(t, loaded)
	assert.Equal(t, 1, v)
	_, ok := m.Load("a")
	assert.False(t, ok, "expect key deleted")
}

// TestMapSwap 验证 Swap：
// 不存在的 key 返回零值 + false，已存在的 key 返回旧值 + true
func TestMapSwap(t *testing.T) {
	m := NewMap[string, int]()
	_, loaded := m.Swap("a", 1)
	assert.False(t, loaded, "expect loaded=false for new key")
	prev, loaded := m.Swap("a", 2)
	assert.True(t, loaded)
	assert.Equal(t, 1, prev)
	v, _ := m.Load("a")
	assert.Equal(t, 2, v)
}

// TestMapCompareAndSwap 验证 CompareAndSwap：
// 旧值匹配时替换成功，不匹配时失败
func TestMapCompareAndSwap(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	assert.True(t, m.CompareAndSwap("a", 1, 2), "expect swap success")
	assert.False(t, m.CompareAndSwap("a", 1, 3), "expect swap fail when old not match")
	v, _ := m.Load("a")
	assert.Equal(t, 2, v)
}

// TestMapCompareAndDelete 验证 CompareAndDelete：
// 值匹配时删除成功，不匹配时不删
func TestMapCompareAndDelete(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	assert.False(t, m.CompareAndDelete("a", 2), "expect delete fail when value not match")
	assert.True(t, m.CompareAndDelete("a", 1), "expect delete success")
	_, ok := m.Load("a")
	assert.False(t, ok, "expect key deleted")
}

// TestMapKeysValuesLen 验证 Keys/Values/Len：
// 存入 3 个键值对，键集合和值集合应与写入一致
func TestMapKeysValuesLen(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Store("c", 3)
	assert.Equal(t, 3, m.Len())
	keys := m.Keys()
	assert.Len(t, keys, 3)
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		assert.True(t, keySet[want], "expect key %q in Keys()", want)
	}
	values := m.Values()
	assert.Len(t, values, 3)
	valSet := map[int]bool{}
	for _, v := range values {
		valSet[v] = true
	}
	for _, want := range []int{1, 2, 3} {
		assert.True(t, valSet[want], "expect value %d in Values()", want)
	}
}

// TestMapRangeEarlyStop 验证 Range 的提前终止：
// 回调返回 false 时应停止遍历
func TestMapRangeEarlyStop(t *testing.T) {
	m := NewMap[int, int]()
	for i := range 5 {
		m.Store(i, i)
	}
	count := 0
	m.Range(func(_, _ int) bool {
		count++
		return false
	})
	assert.Equal(t, 1, count, "expect range stop after 1")
}

// TestMapClear 验证 Clear 清空所有数据
func TestMapClear(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Clear()
	assert.Zero(t, m.Len(), "expect len 0 after Clear")
}

// TestMapConcurrent 验证并发读写安全：
// 多 goroutine 同时 Store/Load/Delete，配合 -race 检测数据竞争；
// 每个 key 最终都被自己的 goroutine 删除，结束后 map 必须为空
func TestMapConcurrent(t *testing.T) {
	m := NewMap[int, int]()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Store(i, i)
			m.Load(i)
			m.Delete(i)
		}(i)
	}
	wg.Wait()
	assert.Zero(t, m.Len(), "所有 key 均已删除, expect len 0")
}

// TestMapNilInRangeAndValues 验证 Range/Values/LoadOrStore/LoadAndDelete/Swap
// 遇到 nil 值时返回零值而不 panic 的分支
func TestMapNilInRangeAndValues(t *testing.T) {
	m := NewMap[string, any]()
	m.Store("k", nil)

	v, loaded := m.LoadOrStore("k", "x")
	assert.True(t, loaded)
	assert.Nil(t, v)
	v, loaded = m.Swap("k", "y")
	assert.True(t, loaded)
	assert.Nil(t, v)
	m.Store("k", nil)
	visited := false
	m.Range(func(key string, value any) bool {
		visited = true
		assert.Nil(t, value, "expect nil value in Range")
		return true
	})
	assert.True(t, visited, "expect Range visited")
	for _, val := range m.Values() {
		assert.Nil(t, val, "expect nil in Values")
	}
	v, loaded = m.LoadAndDelete("k")
	assert.True(t, loaded)
	assert.Nil(t, v)
}

// TestMapDeleteMissing 验证删除不存在 key 的幂等性：不 panic 即可
func TestMapDeleteMissing(t *testing.T) {
	m := NewMap[string, int]()
	m.Delete("missing")
	assert.Zero(t, m.Len(), "删除不存在的 key 后 map 应仍为空")
}

// ==== ttl_cache.go ====

// TestTTLCacheSetGet 验证 Set/Get 正常路径：
// 同类型值写入后能直接反射赋值读出
func TestTTLCacheSetGet(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "a", 42)
	var v int
	err := c.Get(ctx, "a", &v)
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
}

// TestTTLCacheGetNotFound 验证 key 不存在时返回 ErrCacheNotFound
func TestTTLCacheGetNotFound(t *testing.T) {
	c := NewTTLCache(time.Minute)
	var v int
	err := c.Get(context.Background(), "missing", &v)
	assert.ErrorIs(t, err, ErrCacheNotFound)
}

// TestTTLCacheGetConvertible 验证可转换类型的反射转换路径：
// 存 int，读到 int64，走 ConvertibleTo 分支
func TestTTLCacheGetConvertible(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "a", 42)
	var v int64
	err := c.Get(ctx, "a", &v)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), v)
}

// TestTTLCacheGetJSONFallback 验证复杂类型走 JSON 序列化回退路径：
// 存 map[string]any，读到结构体，反射无法直接转换，走 json.Marshal/Unmarshal
func TestTTLCacheGetJSONFallback(t *testing.T) {
	type user struct {
		Name string `json:"name"`
	}
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "u", map[string]any{"name": "tom"})
	var u user
	err := c.Get(ctx, "u", &u)
	assert.NoError(t, err)
	assert.Equal(t, "tom", u.Name)
}

// TestTTLCacheGetDestNotPointer 验证 dest 非指针时返回错误
func TestTTLCacheGetDestNotPointer(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "a", 1)
	var v int
	assert.Error(t, c.Get(ctx, "a", v), "expect error when dest is not a pointer")
}

// TestTTLCacheDel 验证 Del 删除后 Get 返回 ErrCacheNotFound
func TestTTLCacheDel(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "a", 1)
	c.Del(ctx, "a")
	var v int
	assert.ErrorIs(t, c.Get(ctx, "a", &v), ErrCacheNotFound)
}

// TestTTLCacheSetNX 验证 SetNX 语义：
// key 不存在时写入成功，已存在时不覆盖旧值
func TestTTLCacheSetNX(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.SetNX(ctx, "a", 1)
	c.SetNX(ctx, "a", 2)
	var v int
	err := c.Get(ctx, "a", &v)
	assert.NoError(t, err)
	assert.Equal(t, 1, v, "expect 1 (not overwritten)")
}

// TestTTLCacheGetJSONMarshalFail 验证 JSON 回退序列化失败的错误路径：
// 存一个 func 值，反射不可赋值不可转换，json.Marshal 必失败，应返回错误而非 panic
func TestTTLCacheGetJSONMarshalFail(t *testing.T) {
	c := NewTTLCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "f", func() {})
	var v string
	assert.Error(t, c.Get(ctx, "f", &v), "expect json marshal error for func value")
}

// TestTTLCacheExpire 验证 TTL 过期语义：
// 用 50ms 短 TTL，轮询等待过期后 Get 返回 ErrCacheNotFound
func TestTTLCacheExpire(t *testing.T) {
	c := NewTTLCache(50 * time.Millisecond)
	ctx := context.Background()
	c.Set(ctx, "a", 1)
	assert.Eventually(t, func() bool {
		var v int
		return errors.Is(c.Get(ctx, "a", &v), ErrCacheNotFound)
	}, 2*time.Second, 10*time.Millisecond, "expect ErrCacheNotFound after expire")
}

// ==== ttl_map.go ====

// TestTTLMap 验证 Store/Load 与过期语义：
// 写入后立即可读，TTL 到期后 Load 返回 false
func TestTTLMap(t *testing.T) {
	cache := NewTTLMap[string, string]()
	defer cache.Dispose()
	cache.Store("a", "1", time.Second)
	v, ok := cache.Load("a")
	assert.True(t, ok)
	assert.Equal(t, "1", v)
	assert.Eventually(t, func() bool {
		_, ok := cache.Load("a")
		return !ok
	}, 2*time.Second, 50*time.Millisecond, "expect key expired")
}

// TestDel 验证后台定时清理会删掉过期数据：
// TTL 设 500ms、清理间隔 50ms，错开两者周期避免时序撞车，
// 轮询等待过期数据必然被清掉，消除时序抖动
func TestDel(t *testing.T) {
	cache := NewTTLMap[string, string]().SetTickerCleanup(50 * time.Millisecond)
	defer cache.Dispose()
	for i := range 10 {
		cache.Store(strconv.Itoa(i), "1", 500*time.Millisecond)
	}
	assert.Equal(t, 10, cache.Len())
	assert.Eventually(t, func() bool {
		return cache.Len() == 0
	}, 2*time.Second, 20*time.Millisecond, "expect expired keys cleaned by ticker")
}

// TestClear 验证固定时间整体清空：
// 固定清理 2 秒才触发，触发前数据必须原样保留，触发后全部清空
func TestClear(t *testing.T) {
	cache := NewTTLMap[string, string]().SwichFixedTimeClear(func() time.Duration { return 2 * time.Second })
	defer cache.Dispose()
	for i := range 10 {
		cache.Store(strconv.Itoa(i), "1", time.Second)
	}
	assert.Equal(t, 10, cache.Len())
	assert.Never(t, func() bool {
		return cache.Len() != 10
	}, time.Second, 50*time.Millisecond, "expect len 10 before fixed clear")
	assert.Eventually(t, func() bool {
		return cache.Len() == 0
	}, 3*time.Second, 50*time.Millisecond, "expect fixed-time clear emptied the map")
}

// TestConcurrentWrite 验证并发 Load 拿到的是同一个值指针：
// 1000 个 goroutine 并发自增同一个计数器，配合 -race 检测数据竞争，
// 结束后计数必须恰好是 1000，缺一次都说明有 goroutine 读丢了数据
func TestConcurrentWrite(t *testing.T) {
	cache := NewTTLMap[string, *atomic.Uint32]()
	defer cache.Dispose()
	var i atomic.Uint32
	cache.Store("a", &i, time.Second)

	var wg sync.WaitGroup
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i, ok := cache.Load("a")
			if ok {
				i.Add(1)
			}
		}()
	}
	wg.Wait()

	v, ok := cache.Load("a")
	assert.True(t, ok)
	assert.Equal(t, uint32(1000), v.Load(), "1000 个 goroutine 应各完成一次自增")
}

// TestTTLMapLoadExpired 验证 Load 的惰性过期分支：
// TTL 极短，后台清理还没跑到时 Load，应就地删除并返回 false
func TestTTLMapLoadExpired(t *testing.T) {
	m := NewTTLMap[string, int]().SetTickerCleanup(time.Hour)
	defer m.Dispose()
	m.Store("a", 1, 20*time.Millisecond)
	assert.Eventually(t, func() bool {
		_, ok := m.Load("a")
		return !ok
	}, time.Second, 10*time.Millisecond, "expect expired key not loaded")
	assert.Zero(t, m.Len(), "expect expired key deleted lazily")
}

// TestTTLMapTickerCleanup 验证 SetTickerCleanup 的定时清理：
// 用 20ms 间隔清理协程，50ms TTL 的数据应被后台删除
func TestTTLMapTickerCleanup(t *testing.T) {
	m := NewTTLMap[string, int]().SetTickerCleanup(20 * time.Millisecond)
	defer m.Dispose()
	m.Store("a", 1, 50*time.Millisecond)
	assert.Eventually(t, func() bool {
		return m.Len() == 0
	}, 2*time.Second, 10*time.Millisecond, "expect ticker cleanup removed expired key")
}

// TestTTLMapLoadOrStore 验证 LoadOrStore：
// 首次返回新值 loaded=false，再次调用返回旧值 loaded=true 且过期时间被刷新
func TestTTLMapLoadOrStore(t *testing.T) {
	m := NewTTLMap[string, int]()
	defer m.Dispose()
	v, loaded := m.LoadOrStore("a", 1, time.Minute)
	assert.False(t, loaded)
	assert.Equal(t, 1, v)
	v, loaded = m.LoadOrStore("a", 2, time.Minute)
	assert.True(t, loaded)
	assert.Equal(t, 1, v)
}

// TestTTLMapDelete 验证 Delete 同时清理数据和过期记录
func TestTTLMapDelete(t *testing.T) {
	m := NewTTLMap[string, int]()
	defer m.Dispose()
	m.Store("a", 1, time.Minute)
	m.Delete("a")
	_, ok := m.Load("a")
	assert.False(t, ok, "expect key deleted")
	// 删除不存在的 key 也应幂等不报错
	m.Delete("missing")
}

// TestTTLMapRange 验证 Range 遍历所有未删除数据
func TestTTLMapRange(t *testing.T) {
	m := NewTTLMap[string, int]()
	defer m.Dispose()
	m.Store("a", 1, time.Minute)
	m.Store("b", 2, time.Minute)
	got := map[string]int{}
	m.Range(func(k string, v int) bool {
		got[k] = v
		return true
	})
	assert.Len(t, got, 2)
	assert.Equal(t, 1, got["a"])
	assert.Equal(t, 2, got["b"])
}

// TestTTLMapFixedTimeClearDispose 验证固定时间清理协程的退出路径：
// 开启 SwichFixedTimeClear 后 Dispose，fixedTimeCleanup 应通过 ctx.Done 正常退出
func TestTTLMapFixedTimeClearDispose(t *testing.T) {
	m := NewTTLMap[string, int]().SwichFixedTimeClear(func() time.Duration { return time.Hour })
	m.Store("a", 1, time.Minute)
	m.Dispose()
	assert.Zero(t, m.Len(), "expect len 0 after Dispose")
}

// TestTTLMapDispose 验证 Dispose 清空数据并停掉清理协程：
// Dispose 后 Len 为 0，重复调用不 panic
func TestTTLMapDispose(t *testing.T) {
	m := NewTTLMap[string, int]()
	m.Store("a", 1, time.Minute)
	m.Dispose()
	assert.Zero(t, m.Len(), "expect len 0 after Dispose")
	// 重复 Dispose 幂等性验证
	m.Dispose()
}
