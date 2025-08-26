// Author: xiexu
// Date: 2022-09-20

package system

import (
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
