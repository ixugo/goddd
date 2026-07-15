package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	gofumpt "mvdan.cc/gofumpt/format"
)

// newGofumptCmd 构建 gofumpt 子命令，内置 gofumpt 格式化能力，透传常用参数。
func newGofumptCmd() *cobra.Command {
	var (
		list      bool
		write     bool
		extraFlag string
		langVer   string
	)
	cmd := &cobra.Command{
		Use:   "gofumpt [flags] [path ...]",
		Short: "内置 gofumpt 格式化工具",
		Long:  "对 Go 源码执行 gofumpt 格式化，无需单独安装 gofumpt。",
		Example: `  goddd gofumpt -w .
  goddd gofumpt -l -w ./internal/...
  goddd gofumpt -w main.go
  goddd gofumpt -extra main.go`,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}
			opts := gofumpt.Options{
				LangVersion: langVer,
			}
			if extraFlag != "" {
				opts.ExtraRules = true
			}

			for _, path := range args {
				if err := processPath(path, list, write, opts); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&list, "list", "l", false, "列出格式不一致的文件")
	cmd.Flags().BoolVarP(&write, "write", "w", false, "将格式化结果写回文件")
	cmd.Flags().StringVar(&extraFlag, "extra", "", "启用额外格式化规则（逗号分隔）")
	cmd.Flags().StringVar(&langVer, "lang", "", "Go 语言版本（如 go1.22）")
	return cmd
}

// processPath 递归处理目标路径下的所有 .go 文件。
func processPath(target string, list, write bool, opts gofumpt.Options) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return processFile(target, list, write, opts)
	}
	return filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return processFile(path, list, write, opts)
	})
}

// processFile 对单个 .go 文件执行 gofumpt 格式化。
func processFile(path string, list, write bool, opts gofumpt.Options) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	formatted, err := gofumpt.Source(src, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if string(formatted) == string(src) {
		return nil
	}

	if list {
		fmt.Println(path)
	}
	if write {
		return os.WriteFile(path, formatted, 0o644)
	}
	if !list {
		_, err = os.Stdout.Write(formatted)
		return err
	}
	return nil
}
