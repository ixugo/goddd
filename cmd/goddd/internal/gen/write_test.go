package gen

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestGenerationWriteFailure 用目录占据输出文件位置，确保写入失败不会被报告为成功。
func TestGenerationWriteFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	const path = "internal/core/sample/model.go"
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := StartFromContentWithProgress("package sample; type User struct { ID int }", "example.com/sample", nil)
	if err == nil {
		t.Fatalf("generation succeeded with an unwritable output: %v", files)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !strings.Contains(err.Error(), path) {
		t.Fatalf("expected wrapped write error containing %s, got %v", path, err)
	}
	if len(files) != 0 {
		t.Fatalf("failed generation reported successful files: %v", files)
	}
}
