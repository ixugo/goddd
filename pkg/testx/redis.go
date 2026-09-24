package testx

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/testx/docker"
)

// Redis 测试容器固定参数,镜像取 alpine 变体以缩短 CI 拉取时间
const (
	redisImage = "redis:7-alpine"
	redisName  = "godddtest-redis"
	redisPort  = "6379"
)

// NewRedis 为测试准备 Redis 容器,返回宿主机访问地址 host:port。
// Redis 不引入客户端依赖,调用方按所用客户端库自行连接;
// 每个测试使用独立临时容器,测试结束自动清理。
func NewRedis(t *testing.T) string {
	t.Helper()

	c, err := docker.StartContainer(redisImage, redisName, redisPort, nil, nil)
	if err != nil {
		t.Fatalf("启动 Redis 测试容器失败: %v", err)
	}
	t.Cleanup(func() {
		if err := docker.StopContainer(c.ID); err != nil {
			t.Logf("删除 Redis 测试容器 %s 失败: %v", c.Name, err)
		}
	})

	waitRedisReady(t, c.ID, c.HostPort)
	return c.HostPort
}

// StopRedis 停止并删除 Redis 测试容器。
// 测试流程无需调用,仅供本地开发欲彻底清理时手动使用。
func StopRedis() error {
	return docker.StopContainer(redisName)
}

// waitRedisReady 容器启动不等于 Redis 就绪,以原生 RESP 协议轮询 PING,
// 避免仅为就绪检查引入 Redis 客户端依赖
func waitRedisReady(t *testing.T, containerID, addr string) {
	t.Helper()
	deadline := time.Now().Add(pingTimeout)
	for {
		err := pingRedis(addr)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 Redis 就绪超时: %v\n容器日志:\n%s", err, docker.DumpContainerLogs(containerID))
		}
		time.Sleep(pingInterval)
	}
}

// pingRedis 以 TCP 直连发送 RESP PING 命令,期望应答 +PONG
func pingRedis(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	if _, err := fmt.Fprint(conn, "PING\r\n"); err != nil {
		return err
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "+PONG") {
		return fmt.Errorf("期望 +PONG 应答, got %q", line)
	}
	return nil
}
