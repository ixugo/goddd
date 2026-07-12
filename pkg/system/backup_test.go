package system

import (
	"fmt"
	"testing"
)

func TestBackup(t *testing.T) {
	t.Skip()
	f := NewFileBackup("./h.txt")
	for i := range 10 {
		f.Write(fmt.Appendf(nil, "%d", i))
	}
}
