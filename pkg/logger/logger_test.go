package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DeRuina/timberjack"
)

// TestLoggerBasic 测试日志基础功能
func TestLoggerBasic(t *testing.T) {
	// SetupSlog 初始化后写入，检查文件存在且内容正确
	t.Run("初始化与写入", func(t *testing.T) {
		dir := t.TempDir()
		cfg := Config{
			Level:      "info",
			FileConfig: FileConfig{Dir: dir, Name: "app.log"},
			Sampler:    Sampler{TickSec: 1, First: 1, Thereafter: 1},
		}
		log, cleanup := SetupSlog(cfg)
		defer cleanup()

		log.Info("测试写入内容")

		data, err := os.ReadFile(filepath.Join(dir, "app.log"))
		if err != nil {
			t.Fatalf("日志文件不存在: %v", err)
		}
		if !strings.Contains(string(data), "测试写入内容") {
			t.Fatalf("内容不匹配: %s", data)
		}
	})

	// 目录自动创建
	t.Run("目录自动创建", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "a", "b", "c")
		w := newRotateWriter(FileConfig{Dir: nested, Name: "test.log"})
		if _, err := w.Write([]byte("data")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(nested, "test.log")); err != nil {
			t.Fatalf("目录未自动创建: %v", err)
		}
		w.Close()
	})

	// SetLevel 覆盖四种级别
	t.Run("日志级别设置", func(t *testing.T) {
		orig := Level.Level()
		defer Level.SetLevel(orig)

		for _, tc := range []struct {
			level string
			want  string
		}{
			{"debug", "debug"},
			{"info", "info"},
			{"warn", "warn"},
			{"error", "error"},
			{"unknown", "info"}, // 默认 info
		} {
			SetLevel(tc.level)
			got := strings.ToLower(Level.Level().String())
			if got != tc.want {
				t.Errorf("SetLevel(%q) = %q, want %q", tc.level, got, tc.want)
			}
		}
	})

	// 默认配置校验
	t.Run("默认配置", func(t *testing.T) {
		cfg := NewDefaultConfig()
		if !cfg.Debug || cfg.Level != "debug" {
			t.Fatalf("默认 Debug/Level 不符: Debug=%v Level=%s", cfg.Debug, cfg.Level)
		}
		if cfg.MaxAge != 7 || cfg.MaxSize != 50 {
			t.Fatalf("默认 MaxAge/MaxSize 不符: MaxAge=%d MaxSize=%d", cfg.MaxAge, cfg.MaxSize)
		}
		if cfg.RotationTime != 12*time.Hour {
			t.Fatalf("默认 RotationTime 不符: %v", cfg.RotationTime)
		}
		if cfg.Dir != "./logs" || cfg.Name != "app.log" {
			t.Fatalf("默认 Dir/Name 不符: Dir=%s Name=%s", cfg.Dir, cfg.Name)
		}
	})

	// ensureNonZero 零值填充
	t.Run("零值填充", func(t *testing.T) {
		cfg := FileConfig{}.ensureNonZero()
		if cfg.Dir != "./logs" || cfg.Name != "app.log" || cfg.MaxAge != 7 || cfg.MaxSize != 50 || cfg.RotationTime != 12*time.Hour {
			t.Fatalf("ensureNonZero 填充异常: %+v", cfg)
		}
		s := Sampler{}.ensureNonZero()
		if s.TickSec != 1 || s.First != 5 || s.Thereafter != 5 {
			t.Fatalf("Sampler ensureNonZero 填充异常: %+v", s)
		}
	})

	// Config 链式调用
	t.Run("链式配置", func(t *testing.T) {
		cfg := NewDefaultConfig().
			SetDir("/tmp/test").
			SetLevel("warn").
			SetDebug(false).
			SetMaxAge(30).
			SetRotationTime(time.Hour).
			SetService("id1", "svc", "1.0").
			SetSampler(Sampler{TickSec: 5, First: 10, Thereafter: 20})

		if cfg.Dir != "/tmp/test" || cfg.Level != "warn" || cfg.Debug {
			t.Fatalf("链式配置不符: %+v", cfg)
		}
		if cfg.MaxAge != 30 || cfg.RotationTime != time.Hour {
			t.Fatalf("MaxAge/RotationTime 不符: %d %v", cfg.MaxAge, cfg.RotationTime)
		}
		if cfg.ServiceID != "id1" || cfg.ServiceName != "svc" || cfg.ServiceVersion != "1.0" {
			t.Fatalf("Service 不符: %+v", cfg)
		}
		if cfg.Sampler.TickSec != 5 || cfg.Sampler.First != 10 || cfg.Sampler.Thereafter != 20 {
			t.Fatalf("Sampler 不符: %+v", cfg.Sampler)
		}
	})
}

// TestRotation 使用 synctest 假时钟测试轮转
func TestRotation(t *testing.T) {
	// 大小轮转：累计写满 MaxSize 后应生成带 -size- 的备份文件
	t.Run("按大小轮转", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "app.log")

			w := &timberjack.Logger{
				Filename:  logPath,
				MaxSize:   1, // 1MB
				LocalTime: true,
			}
			defer w.Close()

			// 分批写入，每次 600KB，累计超 1MB 触发轮转
			chunk := make([]byte, 600*1024)
			for i := range chunk {
				chunk[i] = 'A'
			}
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}
			// 第二次写入时 size+writeLen > max，触发大小轮转
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}

			entries, _ := os.ReadDir(dir)
			found := false
			for _, e := range entries {
				if strings.Contains(e.Name(), "-size") {
					found = true
					break
				}
			}
			if !found {
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("未找到大小轮转文件, 目录内容: %v", names)
			}
		})
	})

	// 时间轮转：synctest 假时钟推进后应生成带 -time- 的备份文件
	t.Run("按时间轮转", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "app.log")

			w := &timberjack.Logger{
				Filename:         logPath,
				MaxSize:          100,
				RotationInterval: 100 * time.Millisecond,
				LocalTime:        true,
			}
			defer w.Close()

			// 首次写入初始化 lastRotationTime
			if _, err := w.Write([]byte("first\n")); err != nil {
				t.Fatal(err)
			}

			// 推进假时钟超过 RotationInterval
			time.Sleep(200 * time.Millisecond)

			// 再次写入触发时间轮转检查
			if _, err := w.Write([]byte("after-time\n")); err != nil {
				t.Fatal(err)
			}

			entries, _ := os.ReadDir(dir)
			found := false
			for _, e := range entries {
				if strings.Contains(e.Name(), "-time") {
					found = true
					break
				}
			}
			if !found {
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Fatalf("未找到时间轮转文件, 目录内容: %v", names)
			}
		})
	})

	// MaxBackups 限制：超出数量的旧备份应被删除
	t.Run("备份数量限制", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "app.log")

			w := &timberjack.Logger{
				Filename:         logPath,
				MaxSize:          1,
				RotationInterval: 100 * time.Millisecond,
				MaxBackups:       2,
				LocalTime:        true,
			}
			defer w.Close()

			// 分批写入，每次 600KB
			chunk := make([]byte, 600*1024)
			for i := range chunk {
				chunk[i] = 'B'
			}

			// 触发 3 次大小轮转，但 MaxBackups=2，最多保留 2 个备份
			for range 4 {
				if _, err := w.Write(chunk); err != nil {
					t.Fatal(err)
				}
			}

			entries, _ := os.ReadDir(dir)
			backupCount := 0
			for _, e := range entries {
				name := e.Name()
				if strings.Contains(name, "-size") || strings.Contains(name, "-time") {
					backupCount++
				}
			}
			if backupCount > 2 {
				t.Fatalf("备份数量 %d 超出 MaxBackups=2", backupCount)
			}
		})
	})
}
