package hook

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMD5AndFileMD5(t *testing.T) {
	file := strings.Repeat("test_md5.txt", 1024*10)

	strMD5 := MD5(file)
	fileMD5, err := SegmentMD5(bytes.NewReader([]byte(file)))
	if err != nil {
		t.Fatalf("FileMD5 error: %v", err)
	}

	if strMD5 != fileMD5 {
		t.Errorf("MD5 mismatch: MD5()=%s, FileMD5()=%s", strMD5, fileMD5)
	}
}

func TestFileMD5MemoryUsage(t *testing.T) {
	cost := UseMemoryUsage()
	defer cost()

	filename := "/Users/xugo/Downloads/out.mp4"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Open file error: %v", err)
	}
	defer file.Close()

	if _, err := SegmentMD5(file); err != nil {
		t.Fatalf("FileMD5 error: %v", err)
	}
}

func TestMD5WithReadAll(t *testing.T) {
	cost := UseMemoryUsage()
	defer cost()

	filename := "/Users/xugo/Downloads/out.mp4"
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	md5str := MD5(string(data))
	t.Logf("MD5(一次性读入)结果: %s", md5str)
}
