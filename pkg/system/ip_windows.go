// Author: xiexu
// Date: 2022-09-20

//go:build windows

package system

import (
	"context"
	"net"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// soExclusiveAddrUse 对应 Windows SDK 的 SO_EXCLUSIVEADDRUSE，x/sys/windows 未导出此常量
const soExclusiveAddrUse = 0x0005

// PortUsed 检测端口  true:已使用;false:未使用
func PortUsed(mode string, port int) bool {
	if port > 65535 || port < 0 {
		return true
	}

	switch strings.ToLower(mode) {
	case "tcp":
		return TCPPortUsed(port)
	default:
		return UDPPortUsed(port)
	}
}

// TCPPortUsed 通过尝试监听指定 TCP 端口来检测其是否已被占用
// Windows 默认允许不同网卡地址重复绑定同一端口，这里设置 SO_EXCLUSIVEADDRUSE
// 让独占语义与 Linux 保持一致，避免漏检
func TCPPortUsed(port int) bool {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, soExclusiveAddrUse, 1)
			})
		},
	}
	conn, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

// UDPPortUsed 通过尝试监听指定 UDP 端口来检测其是否已被占用
func UDPPortUsed(port int) bool {
	addr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort("", strconv.Itoa(port)))
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}
