package testx

import "testing"

// TestNewRedis 验证 Redis 容器可用且 PING 应答正常
func TestNewRedis(t *testing.T) {
	t.Parallel()
	addr := NewRedis(t)
	if addr == "" {
		t.Fatal("期望返回宿主机地址, got 空串")
	}
	if err := pingRedis(addr); err != nil {
		t.Fatalf("PING 失败: %v", err)
	}
}
