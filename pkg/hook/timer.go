package hook

import (
	"context"
	"time"
)

func UseTimer(ctx context.Context, fn func(), nextTime func() time.Duration) {
	timer := time.NewTimer(nextTime())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			fn()
			timer.Reset(nextTime())
		}
	}
}

// NextTimeTomorrow 计算距离明天指定时间还有多久
// hour: 0-23, minute: 0-59, second: 0-59
// 返回距离明天指定时间的时长
func NextTimeTomorrow(hour, minute, second int) time.Duration {
	now := time.Now().AddDate(0, 0, 1)
	tomorrow := now.AddDate(0, 0, 1)
	nextTime := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, minute, second, 0, now.Location())
	return nextTime.Sub(now)
}

// NextTimeWithFirst 创建一个闭包函数，第一次调用返回 firstWait，之后调用返回 fn() 的结果
func NextTimeWithFirst(firstWait time.Duration, fn func() time.Duration) func() time.Duration {
	isFirst := true
	return func() time.Duration {
		if isFirst {
			isFirst = false
			return firstWait
		}
		return fn()
	}
}
