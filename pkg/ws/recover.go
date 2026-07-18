package ws

import (
	"log/slog"
	"runtime/debug"
)

// recoverLog 捕获 panic 并记录错误日志，供各协程顶部 defer 调用。
// 库内协程与业务回调的 panic 不应击垮整个进程，仅留日志供排查。
func recoverLog(msg string, args ...any) {
	if r := recover(); r != nil {
		slog.Error(msg, append(args, "error", r, "stack", debug.Stack())...)
	}
}

// safeGo 在独立协程中执行 fn，fn 内 panic 仅记录日志，不波及进程。
// connect/disconnect 等业务回调经此入口启动，回调由业务方编写，库不能信任其稳定性。
func safeGo(fn func()) {
	go func() {
		defer recoverLog("business callback panic")
		fn()
	}()
}
