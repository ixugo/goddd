package system

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFile(t *testing.T) {
	_ = os.MkdirAll("./test", 0o744)
	for i := range 20 {
		os.WriteFile(filepath.Join("./test", fmt.Sprintf("%d.txt", i)), []byte("123"), os.ModeAppend|os.ModePerm)
	}
	size, err := GetDirSize("./test")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(size, "===", 3*20)
	// if err := CleanOldFiles("./test", 10); err != nil {
	// 	t.Fatal(err)
	// }
	// if err := CleanOldFiles("./test", 10); err != nil {
	// 	t.Fatal(err)
	// }
	// if err := CleanOldFiles("./test", 10); err != nil {
	// 	t.Fatal(err)
	// }
	// RemoveEmptyDirs("./test")
}

func BenchmarkRemoveEmptyDirs(b *testing.B) {
	base := "./test_bench"
	mkTestDirs := func() {
		os.RemoveAll(base)
		os.MkdirAll(base, 0o755)
		for i := 0; i < 100; i++ {
			lvl1 := filepath.Join(base, fmt.Sprintf("d1_%d", i))
			os.MkdirAll(lvl1, 0o755)
		}
	}
	b.ResetTimer()
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	b.Run("RemoveEmptyDirsV2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mkTestDirs()
			b.StartTimer()
			RemoveEmptyDirs(context.Background(), base, start, end)
			b.StopTimer()
		}
	})
	os.RemoveAll(base)
}

func TestRemoveEmptyDirsV2(t *testing.T) {
	base := "./test_bench"
	mkTestDirs := func() {
		os.RemoveAll(base)
		os.MkdirAll(base, 0o755)
		for i := 0; i < 100; i++ {
			lvl1 := filepath.Join(base, fmt.Sprintf("d1_%d", i))
			os.MkdirAll(lvl1, 0o755)
			for j := 0; j < 10; j++ {
				lvl2 := filepath.Join(lvl1, fmt.Sprintf("d2_%d", j))
				os.MkdirAll(lvl2, 0o755)
				for k := 0; k < 10; k++ {
					lvl3 := filepath.Join(lvl2, fmt.Sprintf("d3_%d", k))
					os.MkdirAll(lvl3, 0o755)
					// 随机生成部分文件
					if rand.Float32() < 0.2 {
						os.WriteFile(filepath.Join(lvl3, "file.txt"), []byte("data"), 0o644)
					}
				}
			}
		}
	}

	// 创建目录结构
	mkTestDirs()

	// 删除所有文件
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			os.Remove(path)
		}
		return nil
	})

	// 执行 RemoveEmptyDirsV2
	slog.SetLogLoggerLevel(slog.LevelDebug)
	err := RemoveEmptyDirs(context.Background(), base, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RemoveEmptyDirsV2 failed: %v", err)
	}

	// 检查是否所有目录都被删除
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			t.Errorf("Directory not removed: %s", path)
		}
		return nil
	})
}
