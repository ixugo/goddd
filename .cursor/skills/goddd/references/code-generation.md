# 代码生成与表定义规范

本文档详述使用 `goddd gen` 工具进行 CRUD 代码生成的标准流程、表结构体定义规范、Wire 注入及路由注册方法。

---

## 代码生成核心原则

1. **生成入口**：新建且符合生成器支持范围的 CRUD 使用 `goddd gen` 生成基础代码；维护已有代码时直接修改本次涉及的实现，不为套用模板重新生成整个领域。
2. **表定义归属**：用于生成的输入模型放在 `tables/<domain>/`（如 `tables/user/user.go`）。已有项目沿用其模型来源，不因本技能搬迁全部模型。
3. **主键与时间戳**：标准 CRUD 输入使用单个 `ID`；新表按业务审计需求定义 `CreatedAt`、`UpdatedAt`。现有表缺少时间戳不构成自动迁移数据库的理由。

---

## 操作步骤

### 1. 定义表结构

在 `tables/<domain>/<entity>.go` 下创建模型文件：

```go
package user

import (
    "time"
)

// User 用户表定义
type User struct {
    ID        int64
    Name      string
    Status    int
    CreatedBy string
    Sort      int64
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

#### 随机字符串/UUID 主键示例

若使用全局唯一随机字符串 ID：

```go
package task

import (
    "time"
    "github.com/ixugo/goddd/pkg/uniqueid"
)

type Task struct {
    ID        uniqueid.Core
    Title     string
    Status    int
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 2. 执行生成命令

```bash
goddd gen -f tables/<domain>/<entity>.go
```

执行前确认输出范围：生成器按路径直接写入文件，不合并手写业务逻辑。对已有领域应在隔离目录核对生成差异，不能直接覆盖未提交或手工维护的实现。命令还会整理依赖，并按环境执行格式化和 Wire；核查这些后处理的影响范围。

生成器会输出以下文件：
- `internal/core/<domain>/core.go`（领域 Core 结构体与 Storer 聚合接口）
- `internal/core/<domain>/<entity>.go`（EntityStorer 接口与 Core 业务方法）
- `internal/core/<domain>/<entity>.model.go`（GORM 实体模型）
- `internal/core/<domain>/<entity>.param.go`（List/Create/Update/Get/Delete Input 参数）
- `internal/core/<domain>/stores/<domain>db/db.go`（DB 聚合入口）
- `internal/core/<domain>/stores/<domain>db/<entity>.db.go`（DB 实体 CRUD 实现）
- `internal/core/<domain>/stores/<domain>cache/cache.go`（Cache 聚合入口）
- `internal/core/<domain>/stores/<domain>cache/<entity>.cache.go`（Cache 实体实现）
- `internal/web/api/<domain>.go`（API 协议转换层）

### 3. Wire Provider 注册

先检查 `internal/web/api/provider.go` 中的已有注册；生成器会尝试更新依赖注入，避免重复添加。构造函数按项目装配方式提供，例如：

```go
func NewUserCore(db *gorm.DB) user.Core {
    store := userdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return user.NewCore(store)
}
```

如果使用了 `uniqueid.Core`：

```go
func NewTaskCore(db *gorm.DB, uni uniqueid.Core) task.Core {
    store := taskdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    return task.NewCore(store, uni)
}
```

**派生文件红线**：`internal/app/wire_gen.go` 属于全自动生成的派生代码，**严禁任何手动修改、微调或补丁注入**。如需变更依赖装配，只能修改 provider 定义后执行 `make wire` 重新生成。

### 4. 路由注册

检查 `internal/web/api/api.go` 的 `setupRouter`；缺少本领域注册时补充，已有调用不要重复添加：

```go
RegisterUser(apiGroup, usecase.UserAPI)
```

### 5. 创建领域文档

在领域目录下创建 `internal/core/<domain>/doc.go`，简要说明该领域负责的业务边界与核心模型。

---

## 注意事项

1. **同领域多表**：同一个业务领域内的多个相关结构体应放在同一个 `tables/<domain>/` 文件中生成，它们将共享同一个 `Storer` 聚合接口。
2. **复合主键或无 ID 表**：生成器不能直接完成此类 CRUD。DB 模板的无 ID 分支会生成运行时 panic，其他生成路径仍引用 ID，不能把产物当作可用实现。应先明确该表的查询与写入条件，在获准范围内实现专用 Store。
3. **过滤字段自动生成**：模板对非空 string 和受支持的非零有符号数值字段生成基础等值条件，并排除 ID、时间戳等字段。布尔值、零值筛选、LIKE 和区间条件需要明确业务语义后实现，不能假定每个 Input 字段都会成为过滤条件。
4. **字段标签**：解析器不保留输入字段的 GORM 标签，而是重建标签。列类型、唯一约束、自定义默认值等需求必须逐项核验生成模型，不能通过给输入添加 tag 就宣称完成。
5. **自定义方法**：输入方法会被保留，实体模板还会生成 `TableName()` 和 `CacheKey()`。同一结构体已定义这些方法时会产生重复声明；保留用户的表名与键语义，在隔离产物中只保留一份正确实现，不静默删除输入方法。

## 生成后验证

核对生成差异、表名、字段约束、过滤语义和路由装配，再运行目标 module 的构建、静态检查与相关测试。生成器的成功提示不是编译通过的证明；`cmd/goddd` 是嵌套 module，验证生成器本身不能替代验证生成产物。
