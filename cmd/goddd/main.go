// cmd/goddd/main.go 是 goddd 命令行工具的入口文件，
// 目前只提供 init 子命令，用于从远程 goddd 模板生成新项目。
package main

import (
	"fmt"
	"os"

	"github.com/ixugo/goddd/cmd/goddd/internal/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
