package conf

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// ReloadCallback 配置重载回调，old 为当前生效配置，new 为新读取配置。
// 回调内应自行比对差异并执行重载动作，返回 error 则跳过本次重载。
type ReloadCallback func(old, new *Bootstrap) error

// WatchConfig 轮询配置文件 mtime，检测到变更后防抖 500ms 再重读并执行回调。
// 回调全部成功后将 new 的值覆写到 old（指针不变，内容替换）。
// ctx 取消时退出。
func WatchConfig(ctx context.Context, bc *Bootstrap, callbacks ...ReloadCallback) {
	path := bc.Runtime.ConfigPath
	if path == "" {
		slog.Warn("WatchConfig: ConfigPath 为空，跳过文件监听")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		slog.Error("WatchConfig: 首次 stat 失败，跳过监听", "path", path, "err", err)
		return
	}
	lastMod := info.ModTime()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		info, err := os.Stat(path)
		if err != nil {
			slog.Warn("WatchConfig: stat 失败", "path", path, "err", err)
			continue
		}
		if !info.ModTime().After(lastMod) {
			continue
		}

		// 防抖：编辑器保存可能产生多次写入，等 500ms 取最终态
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return
		}

		// 重读后再取一次 mtime，确保用最终文件
		info, err = os.Stat(path)
		if err != nil {
			slog.Warn("WatchConfig: 防抖后 stat 失败", "path", path, "err", err)
			continue
		}
		lastMod = info.ModTime()

		var newBC Bootstrap
		if err := SetupConfig(&newBC, path); err != nil {
			slog.Error("WatchConfig: 解析配置失败，跳过本次重载", "path", path, "err", err)
			continue
		}

		failed := false
		for _, cb := range callbacks {
			if err := cb(bc, &newBC); err != nil {
				slog.Error("WatchConfig: 回调执行失败", "err", err)
				failed = true
				break
			}
		}
		if failed {
			continue
		}

		newBC.Runtime = bc.Runtime
		*bc = newBC

		slog.Info("WatchConfig: 配置已重载", "path", path)
	}
}
