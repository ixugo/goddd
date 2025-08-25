// Author: xiexu
// Date: 2022-09-20

package system

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LocalIP 获取本地IP地址
func LocalIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 3*time.Second)
	if err != nil {
		return ""
	}
	host, _, _ := net.SplitHostPort(conn.LocalAddr().(*net.UDPAddr).String())
	if host != "" {
		return host
	}
	iip := strings.Split(localIP()+"/", "/")
	if len(iip) >= 2 {
		return iip[0]
	}
	return ""
}

// localIP 获取本地 IP，遇到虚拟 IP 有概率不准确
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	ip := ""
	for _, v := range addrs {
		net, ok := v.(*net.IPNet)
		if !ok {
			continue
		}
		if net.IP.IsMulticast() || net.IP.IsLoopback() || net.IP.IsLinkLocalMulticast() || net.IP.IsLinkLocalUnicast() {
			continue
		}
		if net.IP.To4() == nil {
			continue
		}

		ip = v.String()
	}
	return ip
}

// PortUsed 检测端口  true:已使用;false:未使用
func PortUsed(mode string, port int) bool {
	if port > 65535 || port < 0 {
		return true
	}

	switch strings.ToLower(mode) {
	case "tcp":
		return tcpPortUsed(port)
	default:
		return udpPortUsed(port)
	}
}

func tcpPortUsed(port int) bool {
	addr, _ := net.ResolveTCPAddr("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	conn, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

func udpPortUsed(port int) bool {
	addr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort("", strconv.Itoa(port)))
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

// ExternalIP 获取公网 IP
func ExternalIP() (string, error) {
	const link = "aHR0cHM6Ly9hcGkubGl2ZS5iaWxpYmlsaS5jb20vY2xpZW50L3YxL0lwL2dldEluZm9OZXc="

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 8 * time.Second}
			return d.DialContext(ctx, "udp4", "8.8.8.8:53")
		},
	}
	dialer := &net.Dialer{
		Resolver: resolver,
		Timeout:  30 * time.Second,
	}
	c := http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint
			},
		},
	}
	url, _ := base64.StdEncoding.DecodeString(link)
	resp, err := c.Get(string(url))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var v externalResponse
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return "", err
	}
	if v.Code != 0 {
		return "", fmt.Errorf("code: %d, msg: %s", v.Code, v.Message)
	}
	return v.Data.Addr, nil
}

type externalResponse struct {
	Code    int          `json:"code"`
	Msg     string       `json:"msg"`
	Message string       `json:"message"`
	Data    ExternalData `json:"data"`
}

type ExternalData struct {
	Addr      string `json:"addr"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}
