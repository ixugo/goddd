package hook

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/assert"
)

// ==== array.go ====

func TestReverse(t *testing.T) {
	// 验证正常路径：元素顺序被完全反转
	got := Reverse([]int{1, 2, 3, 4})
	assert.Equal(t, []int{4, 3, 2, 1}, got)

	// 验证空切片不会 panic，返回仍是空切片
	assert.Empty(t, Reverse([]int{}))

	// 验证原切片不被修改，Reverse 内部是克隆后反转
	src := []int{1, 2, 3}
	Reverse(src)
	assert.Equal(t, []int{1, 2, 3}, src, "原切片不应被修改")
}

func TestUnique(t *testing.T) {
	// 全部唯一时返回 true
	assert.True(t, Unique([]int{1, 2, 3}))
	// 存在重复时返回 false
	assert.False(t, Unique([]int{1, 2, 2}))
	// 空切片没有任何重复，应返回 true
	assert.True(t, Unique([]int{}))
}

func TestAny(t *testing.T) {
	// 命中条件返回 true
	assert.True(t, Any([]int{1, 2, 3}, func(v int) bool { return v == 2 }))
	// 全部不命中返回 false
	assert.False(t, Any([]int{1, 2, 3}, func(v int) bool { return v > 10 }))
	// 空切片必然不命中
	assert.False(t, Any([]int{}, func(v int) bool { return true }))
}

func TestDeduplicationFunc(t *testing.T) {
	// 按自定义 key 去重：长度相同的字符串只保留第一个
	got := DeduplicationFunc([]string{"aa", "bb", "ccc"}, func(s string) string {
		return string(rune(len(s)))
	})
	assert.Equal(t, []string{"aa", "ccc"}, got)

	// 空输入返回空切片而非 nil 异常
	assert.Empty(t, DeduplicationFunc([]string{}, func(s string) string { return s }))
}

func TestDeduplication(t *testing.T) {
	// 重复元素被去除且保持首次出现的顺序
	got := Deduplication(3, 1, 3, 2, 1)
	assert.Equal(t, []int{3, 1, 2}, got)

	// 无入参返回空切片
	assert.Empty(t, Deduplication[int]())
}

// ==== cache.go ====

func TestUseCache(t *testing.T) {
	cacheFn := UseCache(func(i int) (int, error) {
		return i, nil
	})

	for i := range 3 {
		v, ok, _ := cacheFn(i)
		assert.False(t, ok, "首次调用不应命中缓存")
		assert.Equal(t, i, v)
	}

	for i := range 3 {
		v, ok, _ := cacheFn(i)
		assert.True(t, ok, "第二次调用应命中缓存")
		assert.Equal(t, i, v)
	}
}

func TestUseTTLCache(t *testing.T) {
	// 验证 TTL 缓存的三个阶段：首次未命中、TTL 内命中、过期后重新计算
	var calls int
	cacheFn := UseTTLCache(100*time.Millisecond, func(i int) (int, error) {
		calls++
		return i * 2, nil
	})

	// 首次调用必然未命中并回源
	v, hit, err := cacheFn(1)
	assert.NoError(t, err)
	assert.False(t, hit, "首次调用不应命中缓存")
	assert.Equal(t, 2, v)

	// TTL 内第二次调用应命中缓存，不回源
	v, hit, err = cacheFn(1)
	assert.NoError(t, err)
	assert.True(t, hit, "TTL 内应命中缓存")
	assert.Equal(t, 2, v)
	assert.Equal(t, 1, calls, "命中缓存时不应回源")

	// 等待过期后再次调用，应重新回源
	time.Sleep(150 * time.Millisecond)
	v, hit, err = cacheFn(1)
	assert.NoError(t, err)
	assert.False(t, hit, "过期后不应命中缓存")
	assert.Equal(t, 2, calls, "过期后应重新回源")
}

func TestUseTTLCacheError(t *testing.T) {
	// 回源函数报错时：错误要透传，且失败结果不允许写入缓存
	wantErr := errors.New("load error")
	calls := 0
	cacheFn := UseTTLCache(time.Minute, func(i int) (int, error) {
		calls++
		return 0, wantErr
	})

	for range 2 {
		_, hit, err := cacheFn(1)
		assert.ErrorIs(t, err, wantErr)
		assert.False(t, hit, "回源报错时不应命中缓存")
	}
	// 失败结果不入缓存，所以每次都会回源
	assert.Equal(t, 2, calls, "失败结果不入缓存，每次都应回源")
}

// ==== convert.go ====

func TestStringsToInts(t *testing.T) {
	// 正常路径：合法数字字符串被转换
	assert.Equal(t, []int{1, 2, 3}, StringsToInts("1", "2", "3"))

	// 空字符串按函数设计被跳过，不参与转换
	assert.Equal(t, []int{1, 2}, StringsToInts("1", "", "2"))

	// 无入参返回空切片
	assert.Empty(t, StringsToInts())
}

func TestStringsToMap(t *testing.T) {
	// 正常路径：所有非空字符串进入 map
	got := StringsToMap("a", "b")
	assert.Len(t, got, 2)
	_, ok := got["a"]
	assert.True(t, ok, "key a 应存在")

	// 空字符串被跳过
	assert.Len(t, StringsToMap("a", ""), 1)

	// 重复 key 自然去重
	assert.Len(t, StringsToMap("a", "a"), 1)
}

func TestIntsToMap(t *testing.T) {
	// 正常路径 + 重复元素去重
	got := IntsToMap(1, 2, 2, 3)
	assert.Len(t, got, 3)
	_, ok := got[2]
	assert.True(t, ok, "key 2 应存在")

	// 无入参返回空 map
	assert.Empty(t, IntsToMap())
}

func TestIntsToStrings(t *testing.T) {
	// 正常路径：整数逐一转为十进制字符串
	assert.Equal(t, []string{"1", "-2", "300"}, IntsToStrings(1, -2, 300))

	// 无入参返回空切片
	assert.Empty(t, IntsToStrings())
}

// ==== md5.go ====

func TestMD5AndFileMD5(t *testing.T) {
	file := strings.Repeat("test_md5.txt", 1024*10)

	strMD5 := MD5(file)
	fileMD5, err := MD5FromIO(bytes.NewReader([]byte(file)))
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, strMD5, fileMD5, "同一内容的字符串 MD5 与流式 MD5 应一致")
}

func TestFileMD5MemoryUsage(t *testing.T) {
	cost := UseMemoryUsage()
	defer cost()

	// 自建临时文件，不依赖任何本机路径
	// 内容写大一些，让流式读取的内存优势在日志里可见
	filename := filepath.Join(t.TempDir(), "test.bin")
	data := bytes.Repeat([]byte("goddd-md5-stream"), 64*1024)
	err := os.WriteFile(filename, data, 0o644)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	file, err := os.Open(filename)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	defer file.Close()

	got, err := MD5FromIO(file)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	// 实质断言：流式计算结果必须与整串 MD5 一致，证明流式读取没有丢数据
	assert.Equal(t, MD5(string(data)), got)
}

func TestMD5WithReadAll(t *testing.T) {
	cost := UseMemoryUsage()
	defer cost()

	// 自建临时文件，验证一次性读入与流式计算的 md5 结果一致
	filename := filepath.Join(t.TempDir(), "test.bin")
	content := bytes.Repeat([]byte("goddd-md5-readall"), 64*1024)
	err := os.WriteFile(filename, content, 0o644)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	data, err := os.ReadFile(filename)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	// 实质断言：一次性读入的 MD5 必须等于已知正确值，与流式结果互相印证
	assert.Equal(t, MD5(string(content)), MD5(string(data)))
}

// errReader 永远返回错误的 io.Reader，用来覆盖 MD5FromIO 的错误分支
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

func TestMD5FromIOError(t *testing.T) {
	// 读取出错时必须返回非 nil error 且结果为空串
	got, err := MD5FromIO(errReader{})
	assert.Error(t, err)
	assert.Empty(t, got)
}

// TestMD5 原本写在 timing_test.go 里，按被测源码归入 md5.go 一节
func TestMD5(t *testing.T) {
	assert.Equal(t, "219262006d1bdd38c740757b30e2a4e8", MD5("asbd123"))
}

func BenchmarkMD5(b *testing.B) {
	str := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1024*1024)
	s := bytes.NewBuffer([]byte(str))
	a := s.Bytes()

	b.Run("io md5", func(b *testing.B) {
		s := bytes.NewReader(a)
		for b.Loop() {
			s.Seek(0, io.SeekStart)

			MD5FromIO(s)
		}
	})
	b.Run("bytes md5", func(b *testing.B) {
		for b.Loop() {
			MD5FromBytes(a)
		}
	})
	b.Run("str md5", func(b *testing.B) {
		for b.Loop() {
			MD5(str)
		}
	})
}

// ==== pprof.go ====

func TestUsePProf(t *testing.T) {
	// 传入非 nil 的 writer，避免在源码目录下生成 pprof 文件
	// 验证返回的停止函数能正常关闭 profile 并落盘
	path := filepath.Join(t.TempDir(), "cpu.pprof")
	f, err := os.Create(path)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	stop := UsePProf(f)
	// 制造一点 CPU 活动，让 profile 有内容可写
	sum := 0
	for i := range 100000 {
		sum += i
	}
	_ = sum
	stop()

	// profile 停止后文件应有内容
	info, err := os.Stat(path)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	assert.True(t, info.Size() > 0, "pprof 文件不应为空")
}

// ==== timer.go ====

func TestUseTimer(t *testing.T) {
	// 验证定时器会按 nextTime 给定的间隔反复触发 fn
	// 用极短间隔触发若干次后取消 context，确认 UseTimer 正常退出
	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32
	done := make(chan struct{})
	go func() {
		UseTimer(ctx, func() {
			// 触发 3 次后取消，验证循环能响应 ctx.Done
			if count.Add(1) == 3 {
				cancel()
			}
		}, func() time.Duration {
			return time.Millisecond
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("UseTimer should exit after context canceled")
	}

	assert.True(t, count.Load() >= 3, "fn 至少应被调用 3 次，实际 %d 次", count.Load())
}

func TestNextTimeTomorrow(t *testing.T) {
	// 结果必须是正数且不超过 48 小时
	// 源码内部加了两天再相减，正常值应落在 (0, 48h] 区间
	d := NextTimeTomorrow(3, 0, 0)
	assert.True(t, d > 0, "间隔应为正数，实际 %v", d)
	assert.False(t, d > 48*time.Hour, "间隔不应超过 48 小时，实际 %v", d)
}

func TestNextTimeWithFirst(t *testing.T) {
	// 第一次调用必须返回 firstWait，之后返回 fn() 的结果
	// 这是该闭包的核心契约：首次快速触发，后续走自定义逻辑
	next := NextTimeWithFirst(time.Second, func() time.Duration {
		return time.Hour
	})

	assert.Equal(t, time.Second, next(), "首次调用应返回 firstWait")
	assert.Equal(t, time.Hour, next())
	assert.Equal(t, time.Hour, next())
}

// ==== timing.go ====

// TestUseTiming 验证返回的耗时值真实反映函数执行时长
func TestUseTiming(t *testing.T) {
	cost := UseTiming(time.Second)
	time.Sleep(50 * time.Millisecond)
	d := cost()
	assert.True(t, d >= 50*time.Millisecond, "统计耗时不应小于实际睡眠时间，实际 %v", d)
	assert.False(t, d > 2*time.Second, "统计耗时不应远超实际时间，实际 %v", d)
}

// TestUseTimingWithLog 捕获 slog 输出，断言超限走 error 分支、未超限走 debug 分支
func TestUseTimingWithLog(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	// 未超限：应输出 DEBUG 级别日志
	func() {
		defer UseTimingWithLog(time.Second)()
		time.Sleep(10 * time.Millisecond)
	}()
	out := buf.String()
	assert.Contains(t, out, "level=DEBUG")
	assert.Contains(t, out, "msg=timing")

	buf.Reset()

	// 超限：应输出 ERROR 级别日志
	func() {
		defer UseTimingWithLog(time.Millisecond)()
		time.Sleep(20 * time.Millisecond)
	}()
	out = buf.String()
	assert.Contains(t, out, "level=ERROR")
	assert.Contains(t, out, "msg=timing")
}
