package hook

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// UsePProf 使用pprof记录cpu profile
// 如果w为nil，则创建一个文件，文件名称为当前文件的文件名和行号，以及当前时间的年月日
// 返回一个函数，调用该函数可以停止pprof记录
func UsePProf(w io.WriteCloser) func() {
	if w == nil {
		_, file, line, _ := runtime.Caller(1)
		var err error
		w, err = os.OpenFile(fmt.Sprintf("%s:%d-%s.pprof", file, line, time.Now().Format("0102")), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			panic(err)
		}
	}
	if err := pprof.StartCPUProfile(w); err != nil {
		panic(err)
	}
	return func() {
		pprof.StopCPUProfile()
		w.Close()
	}
}
