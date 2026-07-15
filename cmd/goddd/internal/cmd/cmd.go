// Package cmd 注册并管理 goddd 命令行工具的根命令及子命令。
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "v1.7.7"

// NewRootCmd 构建并返回根命令。
func NewRootCmd() *cobra.Command {
	var showVersion bool
	root := &cobra.Command{
		Use:   "goddd",
		Short: "goddd CLI tool",
		Long:  "用于初始化和管理 goddd 项目的命令行工具。",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Println("goddd", version)
				return nil
			}
			return cmd.Help()
		},
	}
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "打印版本号")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newInitCmd())
	root.AddCommand(newGenCmd())
	root.AddCommand(newGofumptCmd())
	return root
}
