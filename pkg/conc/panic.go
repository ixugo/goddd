package conc

import (
	"runtime"
	"strconv"
	"strings"
)

// PanicStack 返回 panic 现场的堆栈文本, 供 defer 的 recover 分支内直接调用。
// 相比 debug.Stack, 剔除栈顶的 recover 闭包与 runtime.gopanic 等运行时帧, 首帧即 panic 发生处。
// 必须在 recover 所在的 deferred 函数中直接调用, 中间不可再隔函数, 否则跳帧数失准。
func PanicStack() []byte {
	// 定长数组留于栈上, 避免 make 切片逃逸到堆
	var pcs [64]uintptr
	// 跳过 runtime.Callers、PanicStack 自身、调用它的 recover 函数共 3 帧;
	// 若 recover 函数被内联, 多跳的一帧恰是 runtime.gopanic, 由下方 runtime 前缀过滤兜底
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	// 逐帧 append 而非 fmt.Fprintf, 避免格式化带来的接口装箱分配
	buf := make([]byte, 0, 2048)
	skipRuntime := true
	for {
		frame, more := frames.Next()
		// 跳过栈顶 runtime 内部帧(gopanic、sigpanic 等), 定位 panic 现场
		if skipRuntime {
			if strings.HasPrefix(frame.Function, "runtime.") {
				if !more {
					break
				}
				continue
			}
			skipRuntime = false
		}
		buf = append(buf, frame.Function...)
		buf = append(buf, '\n', '\t')
		buf = append(buf, frame.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(frame.Line), 10)
		buf = append(buf, " +0x"...)
		buf = strconv.AppendUint(buf, uint64(frame.PC-frame.Entry), 16)
		buf = append(buf, '\n')
		if !more {
			break
		}
	}
	return buf
}
