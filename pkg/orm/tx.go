package orm

import "gorm.io/gorm"

// Tx 事务句柄，用于跨域事务协调。
// Core 层只感知 Commit/Rollback 语义，不接触 gorm。
type Tx interface {
	Commit() error
	Rollback() error
}

type gormTx struct{ db *gorm.DB }

func (t *gormTx) Commit() error   { return t.db.Commit().Error }
func (t *gormTx) Rollback() error { return t.db.Rollback().Error }

// Begin 开启一个事务，返回 Tx 句柄。
func Begin(db *gorm.DB) (Tx, error) {
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &gormTx{db: tx}, nil
}

// Transaction 便捷方法：开启事务，执行 fn，自动 Commit/Rollback。
func Transaction(db *gorm.DB, fn func(Tx) error) error {
	tx, err := Begin(db)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GormDB 从 Tx 中提取底层 *gorm.DB，仅供 Store 层实现使用。
func GormDB(tx Tx) *gorm.DB {
	return tx.(*gormTx).db
}
