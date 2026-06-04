package orm

import (
	"context"

	"gorm.io/gorm"
)

// Deprecated: 请使用 ListWithContext
func FindWithContext[T any](ctx context.Context, db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(ctx, db, out, p, opts...)
}

// Deprecated: 请使用 List
func Find[T any](db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return List(db, out, p, opts...)
}

// Deprecated: 请使用 List
func (t Type[T]) Find(ctx context.Context, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(ctx, t.db, out, p, opts...)
}

// Deprecated: 请使用 Update
func (t Type[T]) Edit(ctx context.Context, model *T, changeFn func(*T) error, opts ...QueryOption) error {
	return UpdateWithContext2(ctx, t.db, model, changeFn, opts...)
}

// Deprecated: 请使用 Create
func (t Type[T]) Add(ctx context.Context, model *T) error {
	return t.db.WithContext(ctx).Create(model).Error
}

// Deprecated: 请使用 Delete
func (t Type[T]) Del(ctx context.Context, model *T, opts ...QueryOption) error {
	return DeleteWithContext(ctx, t.db, model, opts...)
}
