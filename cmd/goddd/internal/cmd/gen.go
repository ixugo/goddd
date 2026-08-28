package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ixugo/goddd/cmd/goddd/internal/gen"
	"github.com/spf13/cobra"
)

// newGenCmd 构建 gen 子命令，将 goddd 的代码生成功能内化。
func newGenCmd() *cobra.Command {
	var (
		filePaths  string
		moduleName string
	)
	cmd := &cobra.Command{
		Use:          "gen",
		Short:        "基于结构体定义生成 DDD 分层代码",
		Long:         "基于 tables 目录下的结构体定义，生成完整的 Core/Store/Cache/API 分层代码。",
		SilenceUsage: true,
		Example: `  goddd gen -f tables/user/user.go
  goddd gen -f tables/user/user.go,tables/task/task.go
  goddd gen -f tables/user/user.go -m github.com/yourname/project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePaths == "" {
				return errors.New("必须通过 -f 指定领域模型文件")
			}
			module := moduleName
			if module == "" {
				module = checkAndExtractModuleName()
			}
			if module == "" {
				printRed("✗ 必须在项目下执行，或者 -m 指定模块名\n")
				os.Exit(1)
			}

			fmt.Println()

			env, err := checkEnvironment()
			if err != nil {
				return err
			}

			files := strings.Split(filePaths, ",")
			generatedCount := 0
			for _, f := range files {
				f = strings.TrimSpace(f)
				if f == "" {
					continue
				}
				stepStart("检查文件 " + f)
				if _, err := os.Stat(f); err != nil {
					stepFail("检查文件")
					return fmt.Errorf("文件不存在: %s", f)
				}
				stepDone("检查文件")

				content, err := os.ReadFile(f)
				if err != nil {
					return fmt.Errorf("读取文件失败: %w", err)
				}

				stepStart("创建领域模型，路由，依赖注入")
				generatedFiles, err := gen.StartFromContentWithProgress(string(content), module, func(label string) {
					printGray("    ✓ %s\n", label)
				})
				if err != nil {
					stepFail("创建领域模型，路由，依赖注入")
					return err
				}
				generatedCount += len(generatedFiles)
			}

			if env.goimports {
				if err := runTool("代码格式化", "goimports", "-w", "."); err != nil {
					return err
				}
			}
			if err := runRequiredTool("整理依赖", "go", "mod", "tidy"); err != nil {
				return err
			}
			if env.wire {
				if err := runTool("依赖注入", "wire", "./..."); err != nil {
					return err
				}
			}

			fmt.Println()
			printGreen("  ✓ %d 个文件已生成，可直接运行\n", generatedCount)
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVarP(&filePaths, "file", "f", "", "领域模型文件路径，多个用逗号分隔（必填）")
	cmd.Flags().StringVarP(&moduleName, "module", "m", "", "Go module 名称（默认从 go.mod 读取）")
	return cmd
}

// runTool 执行外部工具命令，失败时返回错误。
func runTool(label string, args ...string) error {
	return runRequiredTool(label, args...)
}

// runRequiredTool 执行必需的命令，带 spinner 动画，失败时返回错误。
func runRequiredTool(label string, args ...string) error {
	return withSpinner(label, func() error {
		output, err := gen.CommandContext(args...)
		if err != nil {
			if len(output) > 0 {
				fmt.Fprintf(os.Stderr, "%s\n", output)
			}
			return err
		}
		return nil
	})
}

// genEnv 记录环境检测结果。
type genEnv struct {
	goimports bool
	wire      bool
}

// checkEnvironment 在执行生成前检测所需的可选工具。
// 缺少工具时蓝色提示用户是否安装；不安装则该步骤后续跳过。
func checkEnvironment() (genEnv, error) {
	var env genEnv

	if _, err := exec.LookPath("goimports"); err == nil {
		env.goimports = true
	} else {
		fmt.Fprintf(os.Stderr, "\033[34m发现缺少 goimports，是否安装？[y/N]: \033[0m")
		if ok, err := readYesNo(); err != nil {
			return env, err
		} else if ok {
			if err := installTool("golang.org/x/tools/cmd/goimports@latest"); err != nil {
				return env, err
			}
			env.goimports = true
		}
	}

	if _, err := exec.LookPath("wire"); err == nil {
		env.wire = true
	} else {
		fmt.Fprintf(os.Stderr, "\033[34m发现缺少 wire，是否安装？[y/N]: \033[0m")
		if ok, err := readYesNo(); err != nil {
			return env, err
		} else if ok {
			if err := installTool("github.com/google/wire/cmd/wire@latest"); err != nil {
				return env, err
			}
			env.wire = true
		}
	}

	return env, nil
}

// readYesNo 从 stdin 读取 y/n 确认，返回 true 表示用户输入 y/yes。
func readYesNo() (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	fmt.Fprintln(os.Stderr)
	return line == "y" || line == "yes", nil
}

// installTool 使用 go install 安装指定工具。
func installTool(pkg string) error {
	return withSpinner("安装 "+pkg, func() error {
		output, err := gen.CommandContext("go", "install", pkg)
		if err != nil {
			if len(output) > 0 {
				fmt.Fprintf(os.Stderr, "%s\n", output)
			}
			return err
		}
		return nil
	})
}

// checkAndExtractModuleName 从当前目录的 go.mod 中提取 module 名称。
func checkAndExtractModuleName() string {
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return ""
	}
	file, err := os.Open("go.mod")
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
