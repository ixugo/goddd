package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// templateRepo 候选模板仓库。
type templateRepo struct {
	Name string
	URL  string
}

var templateRepos = []templateRepo{
	{Name: "github", URL: "https://github.com/ixugo/goddd.git"},
	{Name: "gitee", URL: "https://gitee.com/ixugo/goddd.git"},
}

const (
	templateBranch = "template-empty"
	// Gitee 先返回后，额外等待 GitHub 的最大时间。
	pingToleranceMs = 1000
	// 探测模板源的总超时时间。
	probeTimeout = 10 * time.Second
	// 克隆模板仓库的总超时时间。
	cloneTimeout = 120 * time.Second
)

// newInitCmd 构建 init 子命令。
func newInitCmd() *cobra.Command {
	var modulePath string
	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "创建并初始化一个新的 goddd 项目",
		Long:  "基于 goddd 模板在当前目录生成新项目。",
		Example: `  goddd init myapp
  goddd init myapp -g github.com/yourname/myapp`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if modulePath != "" && strings.TrimSpace(modulePath) == "" {
				return errors.New("-g/--module 的值不能为空")
			}
			return runInit(cmd.Context(), args[0], modulePath)
		},
	}
	cmd.Flags().StringVarP(&modulePath, "module", "g", "", "自定义 Go module 路径（不可为空）")
	return cmd
}

func stepStart(msg string) { fmt.Printf("  > %s\n", msg) }
func stepDone(msg string)  { printGreen("  ✓ %s\n", msg) }
func stepFail(msg string)  { printRed("  ✗ %s\n", msg) }

// runInit 执行项目初始化逻辑。
func runInit(_ context.Context, name, module string) error {
	fmt.Println()
	overwrite, err := prepareTargetDir(name)
	if err != nil {
		return err
	}

	picked, err := probeWithSpinner()
	if err != nil {
		return err
	}
	repo := picked.URL

	if overwrite {
		err = withSpinnerTimeout("模板初始化", cloneTimeout, func(ctx context.Context) error {
			return cloneToExistingDir(ctx, repo, name)
		})
	} else {
		err = withSpinnerTimeout("模板初始化", cloneTimeout, func(ctx context.Context) error {
			if err := cloneTemplate(ctx, repo, name); err != nil {
				return err
			}
			return cleanupProject(name)
		})
	}
	if err != nil {
		return fmt.Errorf("模板初始化失败: %w", err)
	}

	if module == "" {
		module = name
	}

	err = withSpinner("完整性校验", func() error {
		if err := replaceModule(name, module); err != nil {
			return err
		}
		return runGoModTidy(name)
	})
	if err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}

	err = withSpinner("初始化仓库", func() error {
		return initGitRepo(name)
	})
	if err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	fmt.Println()
	printGreen("  ✓ 项目初始化成功\n")
	fmt.Println()
	fmt.Printf("    cd %s && go run main.go\n", name)
	fmt.Println()
	return nil
}

// withSpinner 执行 fn，标题行保持静态，动画在下一行播放，完成后仅清除动画行。
// probeWithSpinner 探测模板源，带 spinner 动画，成功后打印选中的源名。
func probeWithSpinner() (templateRepo, error) {
	const label = "探测模板源"
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	type result struct {
		repo templateRepo
		err  error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		r, err := pickTemplateRepo(ctx)
		done <- result{r, err}
	}()

	fmt.Printf("  > %s\n", label)
	chars := []rune{'\\', '|', '/', '-'}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case res := <-done:
			fmt.Printf("\r\033[K")
			if res.err != nil {
				printRed("  ✗ %s\n", label)
				return templateRepo{}, fmt.Errorf("探测模板源失败 (耗时 %s): %w\n建议检查网络连接后重试",
					time.Since(start).Truncate(time.Millisecond), res.err)
			}
			printGreen("  ✓ %s %s\n", label, res.repo.Name)
			return res.repo, nil
		case <-ctx.Done():
			fmt.Printf("\r\033[K")
			printRed("  ✗ %s\n", label)
			return templateRepo{}, fmt.Errorf("探测模板源失败 (耗时 %s): %w\n建议检查网络连接后重试",
				time.Since(start).Truncate(time.Millisecond), ctx.Err())
		case <-ticker.C:
			fmt.Printf("  %c\r", chars[i%len(chars)])
			i++
		}
	}
}

func withSpinner(label string, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	fmt.Printf("  > %s\n", label)

	chars := []rune{'\\', '|', '/', '-'}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case err := <-done:
			fmt.Printf("\r\033[K")
			if err != nil {
				printRed("  ✗ %s\n", label)
				return err
			}
			printGreen("  ✓ %s\n", label)
			return nil
		case <-ticker.C:
			fmt.Printf("  %c\r", chars[i%len(chars)])
			i++
		}
	}
}

// withSpinnerTimeout 与 withSpinner 行为一致，但为 fn 提供超时控制。
// 超时后会取消 ctx 并返回错误。
func withSpinnerTimeout(label string, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	fmt.Printf("  > %s\n", label)

	chars := []rune{'\\', '|', '/', '-'}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case err := <-done:
			fmt.Printf("\r\033[K")
			if err != nil {
				printRed("  ✗ %s\n", label)
				return err
			}
			printGreen("  ✓ %s\n", label)
			return nil
		case <-ctx.Done():
			fmt.Printf("\r\033[K")
			printRed("  ✗ %s\n", label)
			return ctx.Err()
		case <-ticker.C:
			fmt.Printf("  %c\r", chars[i%len(chars)])
			i++
		}
	}
}

// cloneToExistingDir 把模板克隆到临时目录，清理后再覆盖到已存在的目标目录。
func cloneToExistingDir(ctx context.Context, repoURL, target string) error {
	tempDir, err := os.MkdirTemp("", "goddd-init-*")
	if err != nil {
		return fmt.Errorf("create temp dir failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := cloneTemplate(ctx, repoURL, tempDir); err != nil {
		return err
	}
	if err := cleanupProject(tempDir); err != nil {
		return err
	}
	if err := clearDir(target); err != nil {
		return fmt.Errorf("clear target dir failed: %w", err)
	}
	if err := moveDirContents(tempDir, target); err != nil {
		return fmt.Errorf("move template contents failed: %w", err)
	}
	return nil
}

// prepareTargetDir 检查目标目录，若目录已存在且非空则询问用户是否覆盖。
// 返回值 overwrite 为 true 表示需要覆盖已有内容。
func prepareTargetDir(name string) (bool, error) {
	info, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s exists and is not a directory", name)
	}
	entries, err := os.ReadDir(name)
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "\033[34m目录 %s 已存在且非空，是否覆盖？[y/N]: \033[0m", name)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	fmt.Fprintln(os.Stderr) // 输入后增加换行
	if line != "y" && line != "yes" {
		printYellow("  ! 已取消\n")
		os.Exit(0)
	}
	return true, nil
}

// clearDir 删除目录下的所有内容，但保留目录本身。
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// moveDirContents 把 src 目录下的所有内容移动到 dst 目录。
func moveDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

type pingResult struct {
	repo templateRepo
	dur  time.Duration
	err  error
}

// pickTemplateRepo 并发探测所有候选源，GitHub 优先。
// 若 GitHub 先返回则直接采用；若 Gitee 先返回，再等 500ms 给 GitHub 机会，
// 超时未到则用 Gitee。两者均失败则报错。
func pickTemplateRepo(ctx context.Context) (templateRepo, error) {
	ch := make(chan pingResult, len(templateRepos))
	for _, repo := range templateRepos {
		go func(r templateRepo) {
			dur, err := probeRepo(ctx, strings.TrimSuffix(r.URL, ".git"))
			ch <- pingResult{repo: r, dur: dur, err: err}
		}(repo)
	}

	var fallback *pingResult
	remaining := len(templateRepos)
	for remaining > 0 {
		select {
		case res := <-ch:
			remaining--
			if res.err != nil {
				continue
			}
			if res.repo.Name == "github" {
				return res.repo, nil
			}
			fallback = &res
			// Gitee 先到，再等 500ms 给 GitHub 机会
			timer := time.NewTimer(time.Duration(pingToleranceMs) * time.Millisecond)
			defer timer.Stop()
			select {
			case gh := <-ch:
				remaining--
				if gh.err == nil && gh.repo.Name == "github" {
					return gh.repo, nil
				}
			case <-timer.C:
			case <-ctx.Done():
			}
			return fallback.repo, nil
		case <-ctx.Done():
			return templateRepo{}, ctx.Err()
		}
	}

	if fallback != nil {
		return fallback.repo, nil
	}
	return templateRepo{}, errors.New("没有可用的模板源")
}

// probeRepo 通过 HTTP HEAD 探测仓库可达性，返回响应耗时。
// 每次调用创建独立 http.Client，防止并发探测共享连接池互相阻塞。
func probeRepo(ctx context.Context, repoURL string) (time.Duration, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, repoURL, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("探测 %s 失败: %w", repoURL, err)
	}
	resp.Body.Close()
	return time.Since(start), nil
}

// cloneTemplate 使用 git 克隆模板仓库的指定分支到目标目录。
func cloneTemplate(ctx context.Context, repoURL, target string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", templateBranch, "--quiet", repoURL, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// cleanupProject 删除模板中不需要的文件和目录。
func cleanupProject(name string) error {
	removes := []string{".git", ".claude", ".cursor", ".DS_Store", "CLAUDE.md", "AGENTS.md"}
	for _, r := range removes {
		_ = os.RemoveAll(filepath.Join(name, r))
	}
	return nil
}

// replaceModule 把模板中的 module 名替换为用户指定的 module 名。
// 仅替换 internal 子路径引用（本地代码）和 go.mod 的 module 声明行，
// 保留 goddd 框架的 pkg/domain 等外部依赖不动。
func replaceModule(name, module string) error {
	const oldModule = "github.com/ixugo/goddd"
	oldInternal := oldModule + "/internal"
	newInternal := module + "/internal"

	return filepath.WalkDir(name, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !shouldReplaceFile(path) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		changed := false

		// 替换 go.mod 的 module 声明行
		if filepath.Base(path) == "go.mod" {
			old := "module " + oldModule
			new := "module " + module
			if strings.Contains(text, old) {
				text = strings.Replace(text, old, new, 1)
				changed = true
			}
		}

		// 替换 .go 文件中的 internal 路径引用
		if strings.Contains(text, oldInternal) {
			text = strings.ReplaceAll(text, oldInternal, newInternal)
			changed = true
		}

		if changed {
			return os.WriteFile(path, []byte(text), 0o644)
		}
		return nil
	})
}

// shouldReplaceFile 判断文件是否需要进行文本替换。
func shouldReplaceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".mod", ".md", ".yaml", ".yml", ".json", ".toml", ".sh", ".mk":
		return true
	default:
		return false
	}
}

// runGoModTidy 在目标目录静默执行 go mod tidy，不打印过程日志。
func runGoModTidy(name string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = name
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// initGitRepo 在目标目录初始化 Git 仓库并创建首次提交。
func initGitRepo(dir string) error {
	cmds := [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "commit", "-m", "chore: first commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", args[0], err, string(output))
		}
	}
	return nil
}
