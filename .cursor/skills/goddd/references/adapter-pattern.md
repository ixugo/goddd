# 适配器模式与跨域解耦

## 设计边界

跨域调用应依赖稳定的能力契约，避免直接访问另一领域的数据库实现。先沿用项目已有接口与注入方式，只有确实需要协议转换、依赖隔离或模型转换时才添加适配器，不为每次跨域调用机械增加一层。

- 消费方需要的窄接口可以定义在本领域 `port.go`；提供方提供通用契约时，可以放在独立的契约包。
- 提供方适配器可放在 `<provider>/<provider>adapter/`，依赖提供方 Core。
- 不让提供方根包反向依赖已引用根包的适配器子包，避免形成 import 环。
- 返回类型随契约归属定义，只包含调用所需字段。不要为了避免重复模型而强迫消费方依赖提供方的持久化模型。
- 使用已有构造函数或 Option 注入；必需依赖不要伪装成可缺省配置。

## 查询契约

例如消息领域需要用户简要信息，可以由消费方定义以下契约。这里的 string ID 仅用于说明，实际类型应与项目模型一致。

```go
type UserBrief struct {
    ID    string
    Name  string
    Cover string
}

type UserBriefProvider interface {
    GetUserBrief(ctx context.Context, userID string) (*UserBrief, error)
}
```

下面片段接续上述契约，依赖 `context`、`fmt`。`UserRecord` 代表提供方返回模型；装配时通过已有构造函数注入非 nil 的 `lookup`，由它调用提供方真实方法，不假定方法名称。入口先按项目契约验证 ID。

```go
type UserRecord struct { ID, DisplayName, AvatarURL string }

type UserAdapter struct {
    lookup func(context.Context, string) (UserRecord, error)
}

// GetUserBrief 隔离提供方模型，同时保留错误链供调用方判断。
func (a UserAdapter) GetUserBrief(ctx context.Context, id string) (*UserBrief, error) {
    record, err := a.lookup(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("读取用户简要信息: %w", err)
    }
    return &UserBrief{
        ID: record.ID, Name: record.DisplayName, Cover: record.AvatarURL,
    }, nil
}
```

仅在契约明确允许、且错误已识别为不存在时，才能转换为缺失结果；超时、权限与数据库错误交给业务调用方处理，不统一吞成 `nil, nil`。
- 批量接口仅在实际需要时添加；定义缺失项和整体失败的语义，并完整实现接口中的方法。
- URL 转换确需 HTTP 请求时，按 [HTTP 上下文说明](with-context.md) 获取；已有绝对 URL 的识别应检查实际协议，不使用宽泛的 `strings.HasPrefix(value, "http")`。

## 跨域事务协调

事务方案取决于一致性要求和数据库边界，不能仅由两个领域的目录关系决定。

| 场景 | 建议 |
|------|------|
| 一个业务用例需要协调多个对等领域 | 在应用编排层或专用协调器中管理事务，避免任一领域承载无关职责 |
| 既有主域负责该用例 | 保留主域编排，通过经过授权的窄接口调用依赖能力 |
| 参与者访问不同数据库或外部服务 | `WithTx` 不能提供跨系统原子性，需要另行明确一致性设计，不在局部任务中自动引入完整分布式方案 |

本仓库的事务契约位于 `pkg/orm/tx.go`：`Begin` 返回 `(orm.Tx, error)`，`Commit` 与 `Rollback` 都返回 error。`orm.GormDB` 只接受本包的事务实现。多个 Store 共用事务前，确认它们与该事务兼容，并且数据位于该事务覆盖的数据库。

执行事务时检查 Begin、每次写操作和 Commit 的错误。失败路径执行 Rollback，并保留原始错误及有意义的回滚错误；不要用无条件 `defer tx.Rollback()` 掩盖错误处理，也不要在提交成功后将重复回滚的错误报告为业务失败。

直接调用 Store 会绕过 Core 中的业务校验。协调器必须复用已有业务规则或调用带事务能力的业务接口，不能因示例使用 Store 就省略权限、余额等约束。事务期间的缓存失效和提交后通知需按现有实现核实，不能假定 `WithTx` 自动处理全部副作用。
