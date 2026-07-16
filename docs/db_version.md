# 用数据库版本号控制 GORM AutoMigrate，让程序启动快起来

在使用 GORM 开发 Go 项目时，`AutoMigrate` 能自动把代码里的模型同步到数据库，省了不少手写 SQL 的麻烦。但它的问题是每次启动都会检查一遍所有表，表多了之后启动速度会明显下降。

goddd 的解决方案很直接：**给数据库结构变更也加一个版本号，只有版本号比数据库里记录的新时才执行 AutoMigrate**。这既保留了自动迁移的便利，又避免了每次启动都做重复检查。设计思路上，goddd 倾向于从简单开始、由浅入深：如果能用一行版本号判断就解决问题，就不必引入一套完整的迁移框架。

## 1. 为什么 AutoMigrate 会越来越慢？

项目初期只有 5 张表，`AutoMigrate` 执行一次只要 1 秒。但随着业务增长，表慢慢增加到 10 张、30 张，启动时间可能变成 10 秒甚至更久。更关键的是：即使代码里完全没有改表结构，GORM 每次启动也会重新检查每张表的字段、索引、约束。假设一次启动多花 10 秒，1000 次启动就是 10000 秒，接近 2 小时 45 分钟。这种每次启动都消耗一点、积少成多的时间成本，才是想优化它的地方。

goddd 的解决方案是**给数据库迁移加一把“版本锁”**：数据库里存一条当前结构版本记录，程序启动时把代码版本号和数据库版本号做对比，只有代码版本号更新时才打开 AutoMigrate 开关。

## 2. 核心判断逻辑

`domain/version/core.go` 里的 `IsAutoMigrate` 负责判断要不要迁移：

```go
func (c Core) IsAutoMigrate(currentVer string) bool {
	var ver Version
	if err := c.store.First(&ver); err != nil {
		// 数据库里还没有版本记录，说明是第一次启动，必须迁移
		isMigrate := true
		*c.IsMigrate = isMigrate
		return isMigrate
	}
	// 比较当前代码版本号和数据库里的版本号
	isMigrate := CompareVersionFunc(currentVer, ver.Version, func(a, b string) bool {
		return a > b
	})
	*c.IsMigrate = isMigrate
	return isMigrate
}
```

两个分支很清晰：没记录就迁移；有记录就取数据库里最新的一条版本号比较，代码版本号更大才迁移。

## 3. 版本号从哪里来？

在 `main.go` 启动时，会把程序构建版本号赋值给 `versionapi.DBVersion`：

```go
var buildVersion = "0.0.1" // 构建版本号

func main() {
	// ... 初始化配置

	versionapi.DBVersion = buildVersion
	versionapi.DBRemark  = gitBranch + "_" + gitHash

	app.Run(&bc)
}
```

这里的 `buildVersion` 在构建时会被 Makefile 通过 `-ldflags -X` 注入为真实的 Git tag 版本号，例如 `v0.0.159`。这样数据库版本号和程序版本号就天然保持一致。

## 4. 迁移开关和版本记录

判断完成后，`domain/version/versionapi/api.go` 会根据结果决定是否开启迁移：

```go
func NewVersionCore(db *gorm.DB) version.Core {
	vdb := versiondb.NewDB(db)
	core := version.NewCore(vdb)
	isOK := core.IsAutoMigrate(DBVersion)
	vdb.AutoMigrate(isOK) // 先确保 versions 表本身存在
	if isOK {
		slog.Info("更新数据库表结构")
		orm.SetEnabledAutoMigrate(true)
	}
	return core
}
```

`vdb.AutoMigrate(isOK)` 会先确保 `versions` 表存在，这样后续才能写入版本记录；`orm.SetEnabledAutoMigrate(true)` 则是业务表迁移的全局开关。后续各个业务模块初始化时都会读取这个开关：

```go
store := tokendb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
```

如果开关是 `false`，`AutoMigrate` 方法会直接返回，不做任何数据库操作。

迁移完成后，新的版本号会写回数据库的 `versions` 表，下次启动才能知道已经迁移过了。这个写回操作放在路由初始化完成之后，只有真正执行了迁移时才会插入新记录：

```go
func (v API) RecordVersion() {
	if !orm.GetEnabledAutoMigrate() {
		return
	}
	if err := v.versionCore.RecordVersion(DBVersion, DBRemark); err != nil {
		slog.Error("RecordVersion", "err", err)
	}
}
```

## 5. 如何触发迁移？

当你改了模型、需要数据库同步时，只需要确保 `DBVersion` 比之前大即可。因为 `main.go` 里已经把 `buildVersion` 赋给 `DBVersion`，所以通常你只需要打一个更高的 Git tag 再重新构建：

```bash
git tag -a v0.0.160 -m "添加用户头像字段"
make build/linux
```

如果本地想强制迁移，可以解开注释：

```go
orm.SetEnabledAutoMigrate(true)
```

## 6. 这样做的好处

1. **启动更快**：没有表结构变更时，启动不再重复检查所有表。
2. **减少数据库压力**：避免每次启动都发起大量元数据查询。
3. **变更可追溯**：`versions` 表记录了每次迁移的版本号和说明。
4. **与发版流程自然结合**：版本号来自 Git tag，迁移时机和软件版本对齐。

## 7. 这个方案不适合所有场景

`versionapi` 适合大多数中小型项目，但遇到这些情况时就显得力不从心了：

- **数据库用了分区表**：GORM 的 `AutoMigrate` 对分区表支持有限，容易出错。
- **DBA 管控严格**：生产环境由 DBA 统一执行 SQL，程序不能自己改表结构。
- **多团队协作**：不同服务共用同一张表时，集中式版本控制可能和发布节奏冲突。
- **已有成熟迁移工具**：比如已经用了 `golang-migrate`、`flyway`、`liquibase` 等。

遇到这些情况，可以直接把 `versionapi` 移除，改用更顺手的方案：

- **SQL 迭代建表**：把每次变更写成独立 SQL 文件，按版本号顺序执行。
- **CLI 参数控制迁移**：给启动命令加 `--migrate`，只有显式指定时才执行迁移：

  ```bash
  ./bin -migrate
  ```

  在 `main.go` 里读取参数后决定是否开启迁移：

  ```go
  migrate := flag.Bool("migrate", false, "是否执行数据库迁移")
  flag.Parse()
  if *migrate {
      orm.SetEnabledAutoMigrate(true)
  }
  ```
- **`gormigrate`**：和 goddd 思路最接近的现成库，在数据库里记录迁移版本，只跑没跑过的迁移，还支持回滚：

  ```go
  m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
      {
          ID: "20260101001",
          Migrate: func(tx *gorm.DB) error {
              return tx.AutoMigrate(&User{})
          },
          Rollback: func(tx *gorm.DB) error {
              return tx.Migrator().DropTable("users")
          },
      },
  })
  if err := m.Migrate(); err != nil {
      log.Fatal(err)
  }
  ```

- **Atlas + atlas-provider-gorm**：GORM 官方集成的 schema-as-code 工具，支持从 GORM 模型生成版本化迁移文件。

所以说，goddd 的 `versionapi` 并不是要重新发明轮子，而是给了一个最简实现：用一个版本号决定要不要执行 `AutoMigrate`。当你的项目变大、迁移场景变复杂时，换成 `gormigrate`、`golang-migrate`、Atlas 等现成工具是完全合理的。

需要强调的是：**goddd 脚手架里的 `versionapi` 只是一个引子，不是为了面面俱到**。它的价值是告诉你“可以这样控制 AutoMigrate”，而不是要求你所有项目都必须这么用。当你发现它不适合当前场景时，随时可以按照团队习惯换成平替方案。

## 关于 goddd

[goddd](https://github.com/ixugo/goddd) 是一个 AI 驱动的 Go 语言脚手架，基于领域驱动设计（DDD）思想搭建，提供了一套适合中小项目快速启动的工程结构和最佳实践。

如果你想了解更多细节，可以访问：

- 官方文档站点：https://goddd.golang.space/
- GitHub 仓库：https://github.com/ixugo/goddd
