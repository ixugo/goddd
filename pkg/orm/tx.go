package orm

import (
	"fmt"

	"gorm.io/gorm"
)

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

// GormDB 从 Tx 中提取底层 *gorm.DB，仅供 Store 层实现使用。
func GormDB(tx Tx) *gorm.DB {
	gt, ok := tx.(*gormTx)
	if !ok {
		panic(fmt.Sprintf("orm.GormDB: unsupported Tx type %T, expected *gormTx", tx))
	}
	return gt.db
}
