package conc

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

// panicBoom 固定的 panic 现场, 供 PanicStack 测试断言首帧位置
//
//go:noinline
func panicBoom() { panic("boom") }

func TestPanicStack(t *testing.T) {
	var stack []byte
	func() {
		defer func() {
			if recover() != nil {
				stack = PanicStack()
			}
		}()
		panicBoom()
	}()

	s := string(stack)
	first := s[:strings.Index(s, "\n")]
	if !strings.Contains(first, "conc.panicBoom") {
		t.Fatalf("首帧应为 panic 现场 panicBoom, 实际: %s", first)
	}
	if strings.Contains(s, "gopanic") {
		t.Fatalf("堆栈不应含 runtime.gopanic 噪声帧:\n%s", s)
	}
}

// TestPanicStackByTrim 验证文本裁切方案同样首帧定位 panic 现场
func TestPanicStackByTrim(t *testing.T) {
	var stack []byte
	func() {
		defer func() {
			if recover() != nil {
				stack = panicStackByTrim()
			}
		}()
		panicBoom()
	}()

	s := string(stack)
	first := s[:strings.Index(s, "\n")]
	if !strings.Contains(first, "conc.panicBoom") {
		t.Fatalf("首帧应为 panic 现场 panicBoom, 实际: %s", first)
	}
	if strings.Contains(s, "gopanic") {
		t.Fatalf("堆栈不应含 runtime.gopanic 噪声帧:\n%s", s)
	}
}

// panicStackByTrim 文本裁切方案对照实现, 仅用于 benchmark 与 PanicStack 比较:
// 取 debug.Stack() 全文, 跳过头部与噪声帧行对
func panicStackByTrim() []byte {
	s := debug.Stack()
	// 跳过 "goroutine N [running]:" 头部行
	i := bytes.IndexByte(s, '\n') + 1
	// 跳过 debug.Stack、panicStackByTrim 自身、调用者(recover 函数)各两行(函数名行 + 文件行);
	// 若调用者被内联, 多跳的两行恰是 runtime.gopanic, 由下方 runtime 前缀过滤兜底
	for range 6 {
		i += bytes.IndexByte(s[i:], '\n') + 1
	}
	// 跳过栈顶 runtime 内部帧(gopanic、sigpanic 等), 定位 panic 现场;
	// 注意 debug.Stack 文本中 runtime.gopanic 被改写为 panic(...)
	for {
		j := bytes.IndexByte(s[i:], '\n')
		line := s[i : i+j]
		funcName := line[:bytes.IndexByte(line, '(')]
		if !bytes.HasPrefix(funcName, []byte("runtime.")) && !bytes.HasPrefix(funcName, []byte("panic")) {
			break
		}
		// 跳过该帧的函数名行与文件行
		i += j + 1
		i += bytes.IndexByte(s[i:], '\n') + 1
	}
	return s[i:]
}

// BenchmarkStack 纯堆栈采集开销对照: PanicStack vs debug.Stack 文本裁切 vs debug.Stack 原文
func BenchmarkStack(b *testing.B) {
	b.Run("PanicStack", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = PanicStack()
		}
	})
	b.Run("debug.Stack裁切", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = panicStackByTrim()
		}
	})
	b.Run("debug.Stack", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = debug.Stack()
		}
	})
}

// recoverWithStack 模拟真实用法: panic 后在 recover 分支采集堆栈
//
//go:noinline
func recoverWithStack(collect func() []byte) {
	defer func() {
		if recover() != nil {
			_ = collect()
		}
	}()
	panicBoom()
}

// BenchmarkRecoverWithStack 含 panic/recover 全链路开销对照
func BenchmarkRecoverWithStack(b *testing.B) {
	b.Run("PanicStack", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			recoverWithStack(PanicStack)
		}
	})
	b.Run("debug.Stack", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			recoverWithStack(debug.Stack)
		}
	})
}
