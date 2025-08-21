package hook

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"unsafe"
)

// MD5 计算 md5
func MD5(s string) string {
	b := md5.Sum(unsafe.Slice(unsafe.StringData(s), len(s)))
	return hex.EncodeToString(b[:])
}

// SegmentMD5 通过缓冲区分段计算 md5
func SegmentMD5(r io.Reader) (string, error) {
	h := md5.New()
	buf := make([]byte, 8*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
