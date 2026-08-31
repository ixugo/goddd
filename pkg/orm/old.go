package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// 以下函数/类型已弃用，仅保留向后兼容。
// 新代码请使用 goddd 生成的 Store 层 + WithTx 模式。
// ============================================================

// Deprecated: 请使用 JSONValueScanner
type Scaner = sql.Scanner

// Deprecated: 请使用 Store 内部直接 db.WithContext(ctx).Take(model)
func First(db *gorm.DB, out any, opts ...QueryOption) error {
	return FirstWithContext(context.TODO(), db, out, opts...)
}

// Deprecated: changeFn 应返回 error，请使用 Store 内部 Transaction + Take + Save
func Update[T any](db *gorm.DB, model *T, changeFn func(*T), opts ...QueryOption) error {
	return UpdateWithContext(context.TODO(), db, model, changeFn, opts...)
}

// Deprecated: changeFn 应返回 error，请使用 UpdateWithContext2
func UpdateWithContext[T any](ctx context.Context, db *gorm.DB, model *T, changeFn func(*T), opts ...QueryOption) error {
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
		changeFn(model)
		return tx.Save(model).Error
	})
}

// Deprecated: 请使用 WithTx 模式替代 Session
func UpdateWithSession[T any](tx *gorm.DB, model *T, fn func(*T) error, opts ...QueryOption) error {
	if len(opts) == 0 {
		panic("where is empty")
	}
	{
		tx := tx.Clauses(clause.Locking{Strength: "UPDATE"})
		for _, opt := range opts {
			tx = opt(tx)
		}
		if err := tx.Take(model).Error; err != nil {
			return err
		}
	}
	if err := fn(model); err != nil {
		return err
	}
	return tx.Save(model).Error
}

// Deprecated: 请使用 Store 内部直接 db.Clauses(clause.Returning{}).Delete(model)
func Delete(db *gorm.DB, model any, opts ...QueryOption) error {
	return DeleteWithContext(context.TODO(), db, model, opts...)
}

// Deprecated: 请使用 Store 内部直接 db.Clauses(clause.Returning{}).Delete(model)
func DeleteWithContext(ctx context.Context, db *gorm.DB, model any, opts ...QueryOption) error {
	if len(opts) == 0 {
		return fmt.Errorf("where is empty")
	}
	db = db.Clauses(clause.Returning{})
	for _, opt := range opts {
		db = opt(db)
	}
	return db.WithContext(ctx).Delete(model).Error
}

// Deprecated: 请使用 goddd 生成的 Store 实现
type Type[T any] struct {
	db *gorm.DB
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func NewType[T any](db *gorm.DB) Type[T] {
	return Type[T]{db: db}
}

// Deprecated: 请使用 Store.GetByID
func (t Type[T]) Get(ctx context.Context, out *T, opts ...QueryOption) error {
	return FirstWithContext(ctx, t.db, out, opts...)
}

// Deprecated: 请使用 Store.Update
func (t Type[T]) Update(ctx context.Context, model *T, changeFn func(*T) error, opts ...QueryOption) error {
	return UpdateWithContext2(ctx, t.db, model, changeFn, opts...)
}

// Deprecated: 请使用 Store.Create
func (t Type[T]) Create(ctx context.Context, model *T) error {
	return t.db.WithContext(ctx).Create(model).Error
}

// Deprecated: 请使用 Store.Update
func (t Type[T]) Edit(ctx context.Context, model *T, changeFn func(*T) error, opts ...QueryOption) error {
	return UpdateWithContext2(ctx, t.db, model, changeFn, opts...)
}

// Deprecated: 请使用 Store.Create
func (t Type[T]) Add(ctx context.Context, model *T) error {
	return t.db.WithContext(ctx).Create(model).Error
}

// Deprecated: 请使用 Store.Delete
func (t Type[T]) Delete(ctx context.Context, model *T, opts ...QueryOption) error {
	return DeleteWithContext(ctx, t.db, model, opts...)
}

// Deprecated: 请使用 Store.Delete
func (t Type[T]) Del(ctx context.Context, model *T, opts ...QueryOption) error {
	return DeleteWithContext(ctx, t.db, model, opts...)
}

// Deprecated: 请使用 Store.List
func (t Type[T]) Find(ctx context.Context, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(ctx, t.db, out, p, opts...)
}

// Deprecated: 请使用 Store.List
func (t Type[T]) List(ctx context.Context, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(ctx, t.db, out, p, opts...)
}

// Deprecated: 请使用 goddd 生成代码，而非内嵌此接口
type Universal[T any] interface {
	Get(context.Context, *T, ...QueryOption) error
	Edit(context.Context, *T, func(*T) error, ...QueryOption) error
	Del(context.Context, *T, ...QueryOption) error
	Add(context.Context, *T) error
	Find(context.Context, *[]*T, Pager, ...QueryOption) (int64, error)
	Delete(context.Context, *T, ...QueryOption) error
	Create(context.Context, *T) error
	List(context.Context, *[]*T, Pager, ...QueryOption) (int64, error)
	Update(context.Context, *T, func(*T) error, ...QueryOption) error
}

// Deprecated: 请使用 WithTx 模式
type UniversalSession[T any] interface {
	Session(ctx context.Context, changeFns ...func(*gorm.DB) error) error
	EditWithSession(tx *gorm.DB, model *T, changeFn func(*T) error, opts ...QueryOption) error
}

// Deprecated: 请使用 ListWithContext
func FindWithContext[T any](ctx context.Context, db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(ctx, db, out, p, opts...)
}

// Deprecated: 请使用 ListWithContext
func Find[T any](db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(context.TODO(), db, out, p, opts...)
}

// Deprecated: 请在 Store 内直接实现分页查询逻辑
type Pager interface {
	Limit() int
	Offset() int
}

// Deprecated: 请在 Store 内直接实现分页查询逻辑
func List[T any](db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	return ListWithContext(context.TODO(), db, out, p, opts...)
}

// Deprecated: 请在 Store 内直接实现分页查询逻辑
func ListWithContext[T any](ctx context.Context, db *gorm.DB, out *[]*T, p Pager, opts ...QueryOption) (int64, error) {
	limit := 9999
	offset := 0
	if p != nil {
		limit = p.Limit()
		offset = p.Offset()
	}
	db = db.Model(new(T)).WithContext(ctx)
	for _, opt := range opts {
		db = opt(db)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil || total <= 0 {
		return total, err
	}
	return total, db.Limit(limit).Offset(offset).Find(out).Error
}

// Deprecated: 请使用 Query.Select
func Select(columns any) QueryOption {
	return func(d *gorm.DB) *gorm.DB {
		return d.Select(columns)
	}
}

// Deprecated: 请使用 Query.OrderBy
func OrderBy(value any) QueryOption {
	return func(d *gorm.DB) *gorm.DB {
		return d.Order(value)
	}
}

// Deprecated: 请在 Store 直接调用 db.Unscoped()
func Unscoped() QueryOption {
	return func(d *gorm.DB) *gorm.DB {
		return d.Unscoped()
	}
}

// Deprecated: 请使用 goddd 生成的 Store 实现
type Engine struct {
	db *gorm.DB
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func NewEngine(db *gorm.DB) Engine {
	return Engine{db: db}
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) InsertOne(model Tabler) error {
	return e.db.Create(model).Error
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) DeleteOne(model Tabler, opts ...Option) error {
	db := e.db.Model(model)
	if len(opts) == 0 {
		return fmt.Errorf("没有指定删除参数")
	}
	for i := range opts {
		opts[i](db)
	}
	return db.Delete(model).Error
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) UpdateOne(model Tabler, id int, data map[string]any) error {
	db := e.db.Model(model)
	WithID(id)(db)
	err := db.Updates(data).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicatedKey
	}
	return err
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) FirstOrCreate(b any) (bool, error) {
	tx := e.db.FirstOrCreate(b)
	return tx.RowsAffected == 1, tx.Error
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) Find(model Tabler, bean any, opts ...Option) (total int64, err error) {
	db := e.db.Model(model)
	for i := range opts {
		opts[i](db)
	}
	err = db.Scan(bean).Limit(-1).Offset(-1).Count(&total).Error
	return
}

// Deprecated: 请使用 goddd 生成的 Store 实现
func (e Engine) NextSeq(model Tabler) (nextID int, err error) {
	db := e.db.Model(model)
	err = db.Raw(fmt.Sprintf(`SELECT nextval('%s_id_seq'::regclass)`, model.TableName())).Scan(&nextID).Error
	return
}

// Deprecated: 请使用 QueryOption func(*gorm.DB) *gorm.DB（返回新 DB 实例的正确链式语义）
type Option func(*gorm.DB)

// Deprecated: 请使用 Store 直接 db.Where("id=?", id)
func WithID(id int) Option {
	return func(d *gorm.DB) {
		d.Where("id=?", id)
	}
}

// Deprecated: 请使用 Pager 接口
func WithLimit(limit, offset int) Option {
	return func(d *gorm.DB) {
		if limit > 0 {
			d.Limit(limit)
		}
		if offset > 0 {
			d.Offset(offset)
		}
	}
}

// Deprecated: 请在 Store 内直接编写时间范围查询
func WithCreatedAt(startAt, endAt int64) Option {
	return func(d *gorm.DB) {
		if startAt > 0 {
			start := time.Unix(startAt, 0)
			d.Where("created_at >= ?", start.Format(time.DateTime))
		}
		if endAt > 0 {
			end := time.Unix(endAt, 0)
			d.Where("created_at < ?", end.Format(time.DateTime))
		}
	}
}
