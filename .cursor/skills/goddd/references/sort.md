# 排序功能实现方案

实现拖拽重排序：接收有序 ID 数组，重新分配 sort 值，不影响未传入的记录。

## 设计思路

将现有 sort 值收集后升序排列，再按用户期望的 ID 顺序重新分配，保证排序值不冲突、不影响其他记录。

## 1. 参数定义

```go
type SortXxxInput struct {
    IDs []int64 `json:"ids"`
}
```

## 2. Store 层 — 批量更新

```go
type SortItem struct {
    ID   int64
    Sort int64
}

func (d Xxx) UpdateSortBatch(ctx context.Context, items []SortItem) error {
    return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        for _, item := range items {
            if err := tx.Model(&Xxx{}).
                Where("id=?", item.ID).
                Update("sort", item.Sort).Error; err != nil {
                return err
            }
        }
        return nil
    })
}
```

## 3. Core 层 — 排序逻辑

```go
func (c Core) SortXxx(ctx context.Context, in *SortXxxInput) error {
    if len(in.IDs) == 0 {
        return reason.ErrBadRequest.SetMsg("ids 不能为空")
    }

    items := make([]*Xxx, 0, len(in.IDs))
    query := orm.NewQuery(1).Where("id IN ?", in.IDs)
    if _, err := c.store.Xxx().Find(ctx, &items, web.NewPagerFilterMaxSize(), query.Encode()...); err != nil {
        return reason.ErrDB.Withf(`Find items err[%s]`, err.Error())
    }

    if len(items) != len(in.IDs) {
        return reason.ErrBadRequest.SetMsg("部分 ID 不存在")
    }

    sorts := make([]int64, 0, len(items))
    for _, item := range items {
        sorts = append(sorts, item.Sort)
    }
    slices.Sort(sorts)

    sortItems := make([]SortItem, 0, len(in.IDs))
    for i, id := range in.IDs {
        sortItems = append(sortItems, SortItem{
            ID:   id,
            Sort: sorts[i],
        })
    }

    if err := c.store.Xxx().UpdateSortBatch(ctx, sortItems); err != nil {
        return reason.ErrDB.Withf(`UpdateSortBatch err[%s]`, err.Error())
    }

    slog.InfoContext(ctx, "排序成功", "ids", in.IDs)
    return nil
}
```

## 4. API 层

```go
func (a XxxAPI) sortXxx(c *gin.Context, in *xxx.SortXxxInput) (any, error) {
    if err := a.xxxCore.SortXxx(c.Request.Context(), in); err != nil {
        return nil, err
    }
    return gin.H{"message": "排序成功"}, nil
}
```

路由注册：

```go
group.PUT("/sort", web.WrapH(api.sortXxx))
```

## 注意事项

1. Store 接口中声明 `UpdateSortBatch` 方法
2. 查询列表时使用 `OrderBy("sort ASC")` 保证按排序值返回
3. 数据库字段 `sort` 使用 gorm tag `autoIncrement` 自增
4. 导入 `slices` 包用于排序
