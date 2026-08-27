package orm

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QueryOption 查询选项函数
type QueryOption func(*gorm.DB) *gorm.DB

// Query 链式查询条件构建器
type Query struct {
	data []QueryOption
}

// NewQuery 创建查询条件构建器
func NewQuery(l int) *Query {
	return &Query{
		data: make([]QueryOption, 0, l),
	}
}

func (q *Query) Where(query any, args ...any) *Query {
	q.data = append(q.data, Where(query, args...))
	return q
}

func (q *Query) OrderBy(value any) *Query {
	q.data = append(q.data, func(d *gorm.DB) *gorm.DB {
		return d.Order(value)
	})
	return q
}

func (q *Query) Select(columns any) *Query {
	q.data = append(q.data, func(d *gorm.DB) *gorm.DB {
		return d.Select(columns)
	})
	return q
}

func (q *Query) Encode() []QueryOption {
	return q.data
}

// Where 查询条件
func Where(query any, args ...any) QueryOption {
	return func(d *gorm.DB) *gorm.DB {
		return d.Where(query, args...)
	}
}

// CountWithContext 通用计数查询
func CountWithContext[T any](ctx context.Context, db *gorm.DB, opts ...QueryOption) (int64, error) {
	var count int64
	tx := db.WithContext(ctx).Model(new(T))
	for _, opt := range opts {
		tx = opt(tx)
	}
	return count, tx.Count(&count).Error
}

// FirstWithContext 按条件查询单条记录（内部使用 Take 避免额外排序）
func FirstWithContext(ctx context.Context, db *gorm.DB, out any, opts ...QueryOption) error {
	if len(opts) == 0 {
		panic("where is empty")
	}
	for _, opt := range opts {
		db = opt(db)
	}
	return db.WithContext(ctx).Take(out).Error
}

// UpdateWithContext2 事务内悲观锁更新（SELECT FOR UPDATE + Save），changeFn 可返回 error 中止事务
func UpdateWithContext2[T any](ctx context.Context, db *gorm.DB, model *T, changeFn func(*T) error, opts ...QueryOption) error {
	if len(opts) == 0 {
		panic("where is empty")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		{
			tx := tx.Clauses(clause.Locking{Strength: "UPDATE"})
			for _, opt := range opts {
				tx = opt(tx)
			}
			if err := tx.Take(model).Error; err != nil {
				return err
			}
		}
		if err := changeFn(model); err != nil {
			return err
		}
		return tx.Save(model).Error
	})
}
