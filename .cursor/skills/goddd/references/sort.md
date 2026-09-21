# 排序：用 WithTx 贯通读写

将选中记录已有的 `sort` 值排序后，按输入 ID 顺序重新分配：`A=10,B=20,C=30` 加输入 `[C,A,B]`，得到 `C=10,A=20,B=30`，未选中记录不变。

## 示例前提

以下为 PostgreSQL + GORM、单个 `catalog` 领域的代码片段。沿用生成的 `Core`、`Storer.Begin()`、`ItemStorer.WithTx()`，Item 包含 `ID`、`ScopeID`、`Sort` 三个 int64 字段。示例直接装配 DB Store，不缓存 Item。

调用前由既有业务入口完成范围授权，并校验 scopeID > 0、IDs 非空且均为正数、无重复、数量不超过项目批量上限。`sort` 在范围内初始互异，且没有即时检查的唯一索引；选中行的修改都遵守行锁协议。此示例不承担插入时排序值分配、跨范围移动及缓存失效。

## Core：事务拥有者

在生成的 `ItemStorer` 中补两个方法；模型仍使用领域内的 `Item`，无需增加排序 DTO：

```go
// ItemStorer 中的增量方法；保留生成的 WithTx 和其他方法。
LockSortItems(context.Context, int64, []int64) ([]Item, error)
SetSort(context.Context, int64, int64, int64) error
```

Core 文件使用 `context`、`fmt`、`log/slog`、`slices`。所有读写使用同一个 `WithTx` 副本，不先查库再开事务：

```go
// SortItems 在一个事务中重排，避免读取排序值后被其他写入穿插。
func (c Core) SortItems(ctx context.Context, scopeID int64, ids []int64) error {
    tx, err := c.store.Begin()
    if err != nil {
        return err
    }
    committed := false
    defer func() {
        if !committed {
            if err := tx.Rollback(); err != nil {
                slog.ErrorContext(ctx, "排序事务回滚失败", "err", err)
            }
        }
    }()
    store := c.store.Item().WithTx(tx)
    if err := reorderItems(ctx, store, scopeID, ids); err != nil {
        return err
    }
    if err := tx.Commit(); err != nil {
        return err
    }
    committed = true
    return nil
}

// reorderItems 复用现有排序值，仅改变本次选中记录的对应关系。
func reorderItems(ctx context.Context, store ItemStorer, scopeID int64, ids []int64) error {
    items, err := store.LockSortItems(ctx, scopeID, ids)
    if err != nil {
        return err
    }
    if len(items) != len(ids) {
        return fmt.Errorf("排序记录不存在或不属于指定范围")
    }
    values := make([]int64, 0, len(items))
    for _, item := range items {
        values = append(values, item.Sort)
    }
    slices.Sort(values)
    if len(slices.Compact(values)) != len(ids) {
        return fmt.Errorf("已有排序值重复")
    }
    for i, id := range ids {
        if err := store.SetSort(ctx, scopeID, id, values[i]); err != nil {
            return err
        }
    }
    return nil
}
```

## DB Store：锁定读取与限定更新

将下列方法加入 `catalogdb.Item`。该接收者和 `db` 字段沿用生成器，使用 `context`、`fmt`、`gorm.io/gorm/clause`，以及本项目领域包 `catalog`。`WithTx` 已将 `d.db` 绑定到 Core 开启的事务，这里不再创建独立事务。

```go
// LockSortItems 按固定顺序锁行，使并发重排使用一致的锁顺序。
func (d Item) LockSortItems(ctx context.Context, scopeID int64, ids []int64) ([]catalog.Item, error) {
    var items []catalog.Item
    err := d.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("scope_id = ? AND id IN ?", scopeID, ids).
        Order("id ASC").Find(&items).Error
    return items, err
}

// SetSort 保留范围条件并检查受影响行数，避免静默漏更新。
func (d Item) SetSort(ctx context.Context, scopeID, id, value int64) error {
    result := d.db.WithContext(ctx).Model(&catalog.Item{}).
        Where("scope_id = ? AND id = ?", scopeID, id).Update("sort", value)
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected != 1 {
        return fmt.Errorf("排序更新行数异常: %d", result.RowsAffected)
    }
    return nil
}
```

## 应用边界与验证

- 入口校验、权限和 API 错误映射复用项目已有机制；上述普通错误用于展示控制流，不规定对外状态码。成功日志放在提交成功之后，错误日志由既有边界统一记录。
- 有 `(scope_id, sort)` 即时唯一约束时，逐条交换会冲突，不能直接套用。先按数据库能力选择并验证可延迟约束或两阶段写入，不能假定临时负数一定可用。
- 行锁仅协调选中记录；若要求与新增、删除、排序值分配协同，所有路径必须遵守同一范围级并发协议。有缓存时还需实现两个新 Store 方法及提交后维护，不能只改 DB 接口。
- 算法 O(n log n)、内存 O(n)、更新 O(n) 次。测试覆盖重排、未选中记录、越界/缺失记录、重复排序值、写入失败回滚；实际 PostgreSQL 上另验两个重排事务的锁等待和唯一约束行为。
