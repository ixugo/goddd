// Author: xiexu
// Date: 2022-09-20

package system

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestPortUsed(t *testing.T) {
	ok := PortUsed("tcp", 8080)
	t.Log(ok)
	ok = PortUsed("tcp", 8001)
	t.Log(ok)
	ok = PortUsed("tcp", 8000)
	t.Log(ok)
	ok = PortUsed("udp", 8000)
	t.Log(ok)
}

func TestExternalIP(t *testing.T) {
	ip, err := ExternalIP()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ip)
}

func TestAA(t *testing.T) {
	s := base64.StdEncoding.EncodeToString([]byte("https://api.live.bilibili.com/client/v1/Ip/getInfoNew"))
	fmt.Println(s)
}
