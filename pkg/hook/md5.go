package hook

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"unsafe"
)

// MD5 计算字符串的 md5
func MD5(s string) string {
	return MD5FromBytes(unsafe.Slice(unsafe.StringData(s), len(s)))
}

// MD5 计算字节数组的 md5
func MD5FromBytes(s []byte) string {
	b := md5.Sum(s)
	return hex.EncodeToString(b[:])
}

// MD5FromIO 计算 io.Reader 的 md5
func MD5FromIO(r io.Reader) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
