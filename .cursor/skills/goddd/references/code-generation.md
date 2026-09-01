# 代码生成与表定义规范

本文档详述使用 `goddd gen` 工具进行 CRUD 代码生成的标准流程、表结构体定义规范、Wire 注入及路由注册方法。

---

## 代码生成核心原则

1. **统一生成入口**：所有单表/多表 CRUD 必须使用 `goddd gen` 自动生成基础代码，严禁手写重复模板。
2. **表定义归属**：所有数据库表模型必须定义在 `tables/<domain>/` 目录下（如 `tables/user/user.go`）。
3. **主键与时间戳**：表结构体必须包含主键（`ID`）、创建时间（`CreatedAt`）、更新时间（`UpdatedAt`）。

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
    ID        int64     `gorm:"primaryKey"`
    Name      string    `gorm:"type:varchar(64);not null"`
    Status    int       `gorm:"type:smallint;default:1"`
    CreatedBy string    `gorm:"type:varchar(64)"`
    Sort      int64     `gorm:"autoIncrement"`
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
    ID        uniqueid.Core `gorm:"primaryKey;type:varchar(32)"`
    Title     string        `gorm:"type:varchar(128);not null"`
    Status    int
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 2. 执行生成命令

```bash
goddd gen -f tables/<domain>/<entity>.go
```

生成器将自动产生并维护以下文件：
- `internal/core/<domain>/core.go`（领域 Core 结构体与 Storer 聚合接口）
- `internal/core/<domain>/<entity>.go`（EntityStorer 接口与 Core 业务方法）
- `internal/core/<domain>/<entity>.model.go`（GORM 实体模型）
- `internal/core/<domain>/<entity>.param.go`（List/Create/Update/Get/Delete Input 参数）
- `internal/core/<domain>/stores/<domain>db/<domain>.db.go`（DB 聚合入口）
- `internal/core/<domain>/stores/<domain>db/<entity>.db.go`（DB 实体 CRUD 实现）
- `internal/core/<domain>/stores/<domain>cache/<domain>.cache.go`（Cache 聚合入口）
- `internal/core/<domain>/stores/<domain>cache/<entity>.cache.go`（Cache 实体实现）
- `internal/web/api/<domain>.go`（API 协议转换层）

### 3. Wire Provider 注册

在 `internal/web/api/provider.go` 中注册构造函数，例如：

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

### 4. 路由注册

在 `internal/web/api/api.go` 的 `setupRouter` 中注册：

```go
RegisterUser(apiGroup, usecase.UserAPI)
```

### 5. 创建领域文档

在领域目录下创建 `internal/core/<domain>/doc.go`，简要说明该领域负责的业务边界与核心模型。

---

## 注意事项

1. **同领域多表**：同一个业务领域内的多个相关结构体应放在同一个 `tables/<domain>/` 文件中生成，它们将共享同一个 `Storer` 聚合接口。
2. **复合主键或无 ID 表**：如果表结构不含单个 `ID` 主键，生成器会生成显式的编译提示，开发者需手动调整 `Update` 与 `Delete` 的查询条件，并在 Store 层显式编写复合主键逻辑。
3. **过滤字段自动生成**：生成器会根据 Input 中的非零字段自动生成 `applyFilter` 基础等值匹配。若包含 LIKE 模糊匹配或区间查询，应在生成的 filter 函数中手动补充。
