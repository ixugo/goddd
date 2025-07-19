// Author: xiexu
// Date: 2022-09-20

package system

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
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
	c := http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint
			},
		},
	}

	resp, err := c.Get("https://api.live.bilibili.com/client/v1/Ip/getInfoNew")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var v bilibiliResponse
	err = json.NewDecoder(resp.Body).Decode(&v)
	return v.Data.Addr, err
}

type bilibiliResponse struct {
	Code    int          `json:"code"`
	Msg     string       `json:"msg"`
	Message string       `json:"message"`
	Data    bilibiliData `json:"data"`
}

type bilibiliData struct {
	Addr      string `json:"addr"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	ISP       string `json:"isp"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

var cli = http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		MaxConnsPerHost:       30,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

const url = "http://whois.pconline.com.cn/ipJson.jsp?json=true&ip="

type Info struct {
	IP          string `json:"ip"`
	Pro         string `json:"pro"`        // 省;安徽省
	ProCode     string `json:"proCode"`    // 省区域代码;340000
	City        string `json:"city"`       // 城市;合肥市
	CityCode    string `json:"cityCode"`   // 城市代码;340100
	Region      string `json:"region"`     // 区域;蜀山区
	RegionCode  string `json:"regionCode"` // 区域代码;340104
	Addr        string `json:"addr"`       // 完整地址;安徽省合肥市蜀山区 电
	RegionNames string `json:"regionNames"`
	Err         string `json:"err"`
}

func IP2Info(ip string) (Info, error) {
	netip := net.ParseIP(ip)
	if netip.IsLoopback() || netip.IsPrivate() {
		return Info{IP: ip, Addr: "内网 IP"}, nil
	}

	resp, err := cli.Get(url + ip)
	if err != nil {
		return Info{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Info{}, fmt.Errorf(resp.Status)
	}
	var out Info
	reader := transform.NewReader(resp.Body, simplifiedchinese.GB18030.NewDecoder())
	err = json.NewDecoder(reader).Decode(&out)
	return out, err
}

// CompareVersionFunc 比较 ip 或 版本号是否一致
func CompareVersionFunc(a, b string, f func(a, b string) bool) bool {
	s1 := versionToStr(a)
	s2 := versionToStr(b)
	if len(s1) != len(s2) {
		return true
	}
	return f(s1, s2)
}

func versionToStr(str string) string {
	var result strings.Builder
	arr := strings.Split(str, ".")
	for _, item := range arr {
		if idx := strings.Index(item, "-"); idx != -1 {
			item = item[0:idx]
		}
		result.WriteString(fmt.Sprintf("%03s", item))
	}
	return result.String()
}
