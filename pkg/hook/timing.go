package hook

import (
	"log/slog"
	"runtime"
	"time"
)

// UseTiming 计算函数花销，超过 limit 用 error 级别记录
// cost := UseTiming(time.Second)
// defer cost()
// 业务操作
func UseTiming(limit time.Duration) func() {
	now := time.Now()
	return func() {
		sub := time.Since(now)
		pc, _, _, _ := runtime.Caller(1)
		fn := runtime.FuncForPC(pc)

		log := slog.With("cost", sub, "caller", fn.Name())
		if sub > limit {
			log.Error("timing")
		} else {
			log.Debug("timing")
		}
	}
}

// UseMemoryUsage 计算内存占用
// cost := UseMemoryUsage()
// defer cost()
func UseMemoryUsage() func() uint64 {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	return func() uint64 {
		runtime.ReadMemStats(&m2)
		memUsed := m2.Alloc - m1.Alloc

		pc, _, _, _ := runtime.Caller(1)
		fn := runtime.FuncForPC(pc)

		log := slog.With("cost(bytes)", float32(memUsed)/1024, "caller", fn.Name())
		log.Debug("memory usage")
		return memUsed
	}
}
