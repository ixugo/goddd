package hook

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMD5AndFileMD5(t *testing.T) {
	file := strings.Repeat("test_md5.txt", 1024*10)

	strMD5 := MD5(file)
	fileMD5, err := MD5FromIO(bytes.NewReader([]byte(file)))
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

	if _, err := MD5FromIO(file); err != nil {
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

func BenchmarkMD5(b *testing.B) {
	str := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1024*1024)
	s := bytes.NewBuffer([]byte(str))
	a := s.Bytes()

	b.Run("io md5", func(b *testing.B) {
		s := bytes.NewReader(a)
		for b.Loop() {
			s.Seek(0, io.SeekStart)

			MD5FromIO(s)
		}
	})
	b.Run("bytes md5", func(b *testing.B) {
		for b.Loop() {
			MD5FromBytes(a)
		}
	})
	b.Run("str md5", func(b *testing.B) {
		for b.Loop() {
			MD5(str)
		}
	})
}
