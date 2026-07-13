package conf

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchConfig_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	initial := Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{Port: 8080},
		},
	}
	if err := WriteConfig(initial, path); err != nil {
		t.Fatal(err)
	}

	var bc Bootstrap
	if err := SetupConfig(&bc, path); err != nil {
		t.Fatal(err)
	}
	bc.Runtime = Runtime{ConfigPath: path}

	var called atomic.Int32
	var gotOldPort, gotNewPort atomic.Int32

	cb := func(old, new *Bootstrap) error {
		gotOldPort.Store(int32(old.Server.HTTP.Port))
		gotNewPort.Store(int32(new.Server.HTTP.Port))
		called.Add(1)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchConfig(ctx, &bc, cb)

	// 等 watcher 启动并完成首次 Stat
	time.Sleep(1500 * time.Millisecond)

	// 修改配置文件：Port 8080 → 9090
	updated := Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{Port: 9090},
		},
	}
	if err := WriteConfig(updated, path); err != nil {
		t.Fatal(err)
	}

	// 等待 watcher 检测到变更（1s 轮询 + 500ms 防抖 + 余量）
	deadline := time.After(5 * time.Second)
	for called.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("超时：watcher 未检测到配置文件变更")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	if v := gotOldPort.Load(); v != 8080 {
		t.Errorf("回调收到的旧 Port = %d, 期望 8080", v)
	}
	if v := gotNewPort.Load(); v != 9090 {
		t.Errorf("回调收到的新 Port = %d, 期望 9090", v)
	}
	// WatchConfig 在回调后会覆写 bc
	if bc.Server.HTTP.Port != 9090 {
		t.Errorf("bc.Server.HTTP.Port = %d, 期望 9090（应被覆写）", bc.Server.HTTP.Port)
	}
	// Runtime 应被保留
	if bc.Runtime.ConfigPath != path {
		t.Errorf("bc.Runtime.ConfigPath 被覆写，期望 %s, 实际 %s", path, bc.Runtime.ConfigPath)
	}
}

func TestWatchConfig_CallbackErrorSkipsReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	initial := Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{Port: 8080},
		},
	}
	if err := WriteConfig(initial, path); err != nil {
		t.Fatal(err)
	}

	var bc Bootstrap
	if err := SetupConfig(&bc, path); err != nil {
		t.Fatal(err)
	}
	bc.Runtime = Runtime{ConfigPath: path}

	var cbCalled atomic.Int32
	failCb := func(_, _ *Bootstrap) error {
		cbCalled.Add(1)
		return os.ErrPermission
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchConfig(ctx, &bc, failCb)

	time.Sleep(1500 * time.Millisecond)

	updated := Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{Port: 9090},
		},
	}
	if err := WriteConfig(updated, path); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for cbCalled.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("超时：watcher 未检测到配置文件变更")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 回调返回 error，bc 不应被覆写
	if bc.Server.HTTP.Port != 8080 {
		t.Errorf("bc.Server.HTTP.Port = %d, 期望 8080（回调失败不应覆写）", bc.Server.HTTP.Port)
	}
}

func TestWatchConfig_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := WriteConfig(Bootstrap{}, path); err != nil {
		t.Fatal(err)
	}

	var bc Bootstrap
	if err := SetupConfig(&bc, path); err != nil {
		t.Fatal(err)
	}
	bc.Runtime = Runtime{ConfigPath: path}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		WatchConfig(ctx, &bc)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("超时：WatchConfig 未在 ctx 取消后退出")
	}
}
