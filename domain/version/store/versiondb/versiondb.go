package versiondb

import (
	"github.com/ixugo/goddd/domain/version"
	"gorm.io/gorm"
)

// DB ...
type DB struct {
	db *gorm.DB
}

// NewDB ...
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// AutoMigrate ...
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(version.Version),
	); err != nil {
		panic(err)
	}
	return d
}

// First 获取最新版本记录
// 如果 versions 表不存在，直接返回 ErrRecordNotFound，避免 gorm 日志打印 "no such table" 错误
func (d DB) First(v *version.Version) error {
	if !d.db.Migrator().HasTable(v) {
		return gorm.ErrRecordNotFound
	}
	return d.db.Order("id DESC").First(v).Error
}

// Add ...
func (d DB) Add(v *version.Version) error {
	return d.db.Create(v).Error
}
