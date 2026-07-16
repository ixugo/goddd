# 构建时自动注入版本号：从 Git tag 到 Go 可执行文件

在发布一个 Go 项目时，我们总希望知道当前跑的是哪个版本、哪次提交、哪个分支。最原始的做法是手动改代码里的版本号，但这种方式很容易忘记更新，导致线上版本和实际代码对不上。

goddd 的做法是：**版本号只写在 Git tag 里，构建时由 Makefile 自动读取并注入到程序中**。这样只要你会打 tag，就不用再操心版本号的事情。

## 1. 整体思路

整个流程可以拆成三步：

1. **用 Git tag 标记版本**：例如 `v1.0.0`、`v1.2.3`，这是唯一的版本来源。
2. **Makefile 自动计算版本字符串**：根据最近的 tag 和距离它的提交次数，生成类似 `v1.0.5` 这样的版本号。
3. **`go build` 构建时注入**：通过 `-ldflags -X` 把版本号写进 Go 程序的变量里。

下面我们就一步步拆开来看。

## 2. 先在代码里留出“注入位”

打开项目根目录下的 `main.go`，可以看到这样一段变量声明：

```go
package main

var (
	buildVersion = "0.0.1" // 构建版本号
	gitBranch    = "dev"   // git 分支
	gitHash      = "debug" // git 提交点哈希值
	release      string    // 发布模式 true/false
	buildTime    string    // 构建时间戳
)
```

这几个变量有两个关键点：

- 它们都是 **`var`** 声明的，不能是 `const`。因为 `go build` 的 `-ldflags -X` 只能在链接阶段修改变量，常量是无法被修改的。
- 它们都位于 **`main` 包**下。Makefile 里注入时用的也是 `main.buildVersion` 这种写法。

在 `main()` 函数里，这些变量又被用到了几个地方：

```go
func main() {
	// ... 初始化配置

	bc.Runtime.BuildVersion = buildVersion

	// 通过 expvar 暴露出去，方便监控和调试
	expvar.NewString("version").Set(buildVersion)
	expvar.NewString("git_branch").Set(gitBranch)
	expvar.NewString("git_hash").Set(gitHash)
	expvar.NewString("build_time").Set(buildTime)

	// 同时作为数据库迁移的版本依据
	versionapi.DBVersion = buildVersion
	versionapi.DBRemark  = gitBranch + "_" + gitHash

	app.Run(&bc)
}
```

也就是说，**只要构建时把 `buildVersion` 等变量改掉，程序内部所有依赖版本号的地方都会自动生效**，不需要手动再去改其他文件。

## 3. Makefile 里怎么算出版本号？

项目的 `Makefile` 中专门有一块 `VERSION` 逻辑，负责从 Git 历史里提取版本信息：

[点击查看 Makefile 版本号计算逻辑](https://github.com/ixugo/goddd/blob/bd4fdaf8af2dda2fb1f221207cf99f8a55d68947/Makefile#L98)

看不懂 Makefile 没关系，我们把它翻译成大白话：

- `RECENT_TAG`：找到当前提交上最近的 Git tag，比如 `v1.0.1`。
- `COMMITS`：数一数从那个 tag 之后到现在，一共又提交了多少次。
- `GIT_VERSION_MAJOR / MINOR / PATCH`：把 tag 拆成主版本号、次版本号、修订号。
- `FINAL_PATCH`：把修订号加上后面的提交次数，得到最终的修订号。
- `VERSION`：把三部分拼起来，例如 `v1.0.12`。

如果没有打过任何 tag，默认从 `v0.0.0` 开始，提交次数直接加到修订号上，变成 `v0.0.15` 这种形式。

你可以直接运行：

```bash
make info
```

它会输出当前计算出来的版本号、分支、提交哈希等信息，方便检查是否正确。

## 4. 构建时把版本号“塞”进程序

版本号算好了，接下来就要在编译时写进程序。关键命令在 `Makefile` 的 `build/local` 目标里：

```makefile
## build/local: 构建本地应用
.PHONY: build/local
build/local:
	$(eval dir := $(BUILD_DIR_ROOT)/$(GOOS)_$(GOARCH))
	@echo 'Building $(VERSION) $(dir)...'
	@rm -rf $(dir)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-trimpath \
		-ldflags="-s -w \
			-X main.buildVersion=$(VERSION) \
			-X main.gitBranch=$(BRANCH) \
			-X main.gitHash=$(HASH_AND_DATE) \
			-X main.buildTimeAt=$(shell date +%s) \
			-X main.release=true \
			" -o=$(dir)/bin ./main.go
	@echo '>>> OK'
```

这里最重要的是 `-ldflags="-X main.buildVersion=$(VERSION)"` 这一行。

`-X` 是 Go 链接器的一个参数，作用是：**在生成可执行文件时，把指定包里的某个字符串变量的值替换掉**。写法是：

```text
-X 包名.变量名=新值
```

因为我们注入的是 `main` 包里的变量，所以写成 `main.buildVersion`、`main.gitBranch` 等。

注意：`-s -w` 是用来裁剪调试信息的，可以让生成的二进制文件更小，和版本注入本身无关。

## 5. 实际使用流程

假设你刚完成了一次功能开发，准备发版：

### 第一步：打 tag

```bash
git tag -a v1.2.0 -m "发布 1.2.0"
git push origin v1.2.0
```

> 建议 tag 名统一用 `v` 开头，例如 `v1.2.0`，这样 Makefile 里的解析规则才能正确工作。

### 第二步：构建

```bash
make build/linux
```

构建完成后，二进制文件会出现在 `build/linux_amd64/bin` 目录下。

### 第三步：验证版本号

程序启动后，你可以通过接口查看当前版本：

```bash
curl http://localhost:8080/health
```

返回值类似：

```json
{
  "version": "v0.0.159",
  "start_at": "2026-07-15T11:13:27.780618606+08:00",
  "git_branch": "dev",
  "git_hash": "c125b7f-260715"
}
```

如果通过配置开启了 pprof，也可以访问：

```bash
curl http://localhost:8080/debug/vars
```

在里面能看到 `version`、`git_branch`、`git_hash`、`build_time` 等字段。

## 6. 如果当前提交不是 tag 怎么办？

这是最常见的情况：你正在开发中，当前提交本身还没有 tag。比如最近的 tag 是 `v1.0.0`，从那以后你又提交了 5 次，那么 `make info` 会显示：

```text
version: v1.0.5
```

也就是说，**中间的开发版本会自动以“最近的 tag + 提交次数”的形式呈现**。等到你正式发布时再打一个 `v1.1.0` 或 `v1.0.5` 的 tag 即可。

## 7. 常见坑和注意事项

1. **变量必须是 `var`，不能是 `const`**
   `-ldflags -X` 只能修改变量，常量注入不了。

2. **变量必须是字符串类型**
   虽然 `release` 看起来是布尔含义，但它在 `main.go` 里声明的是 `string`，后续再用 `strconv.ParseBool` 去解析。

3. **tag 要推送**
   只本地打 tag 不推送到远程，CI/CD 构建时就读不到。发布时记得 `git push origin <tag>`。

4. **Windows 下没有 `bc` 命令**
   Makefile 里特意用 `awk` 代替了 `bc`，就是为了兼容 Windows 的 Git Bash 环境。

5. **注入的是 `main` 包变量**
   如果你想给非 `main` 包的变量注入值，`-X` 后面要写完整的包路径，例如 `-X github.com/ixugo/goddd/internal/conf.buildVersion=xxx`。

## 8. 这样做的好处

- **版本号和代码解耦**：再也不用每次发版都改代码里的字符串。
- **版本号有迹可循**：看到 `v1.0.5` 就能知道它基于 `v1.0.0` 之后第 5 次提交。
- **构建产物可追踪**：每个二进制文件都自带分支、commit hash、构建时间，出了问题方便定位。
- **和 CI/CD 天然适配**：流水线里只需要 `git tag` + `make build/linux` 两步。

## 关于 goddd

[goddd](https://github.com/ixugo/goddd) 是一个 AI 驱动的 Go 语言脚手架，基于领域驱动设计（DDD）思想搭建，提供了一套适合中小项目快速启动的工程结构和最佳实践。

如果你想了解更多细节，可以访问：

- 官方文档站点：https://goddd.golang.space/
- GitHub 仓库：https://github.com/ixugo/goddd
