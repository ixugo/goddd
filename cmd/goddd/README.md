# goddd CLI

用于初始化和管理 goddd 项目的命令行工具。

## 安装

```bash
go install github.com/ixugo/goddd/cmd/goddd@latest
```

## 命令

### init — 创建新项目

```bash
goddd init myapp
goddd init myapp -g github.com/yourname/myapp
```

| 参数 | 说明 |
|------|------|
| `-g` / `--module` | 自定义 Go module 路径，不指定则默认使用项目名 |

init 会并发探测 GitHub 和 Gitee 模板源，优先使用 GitHub；若 Gitee 先返回，会再等待最多 1s 等 GitHub 响应，超时则使用 Gitee。

### gen — 生成 DDD 分层代码

基于 `tables/` 目录下的结构体定义，生成 Core/Store/Cache/API 分层代码。

```bash
goddd gen -f tables/user/user.go
goddd gen -f tables/user/user.go,tables/task/task.go
goddd gen -f tables/user/user.go -m github.com/yourname/project
```

| 参数 | 说明 |
|------|------|
| `-f` / `--file` | 领域模型文件路径，多个用逗号分隔（必填） |
| `-m` / `--module` | Go module 名称，默认从 go.mod 读取 |

gen 执行流程：检查文件 → 创建领域模型/路由/依赖注入 → 代码格式化（goimports） → 整理依赖（go mod tidy） → 依赖注入（wire）。

首次运行时，若缺少 goimports 或 wire 会提示安装。

### gofumpt — 内置代码格式化

无需单独安装 gofumpt，直接使用：

```bash
goddd gofumpt -w .
goddd gofumpt -l -w ./internal/...
goddd gofumpt -w main.go
goddd gofumpt -extra main.go
```

| 参数 | 说明 |
|------|------|
| `-l` / `--list` | 列出格式不一致的文件 |
| `-w` / `--write` | 将格式化结果写回文件 |
| `--extra` | 启用额外格式化规则 |
| `--lang` | 指定 Go 语言版本（如 go1.22） |
