package versionapi

import (
	"log/slog"

	"github.com/ixugo/goddd/domain/version"
	"github.com/ixugo/goddd/domain/version/store/versiondb"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"
)

// 通过修改版本号，来控制是否执行表迁移
var (
	DBVersion = "0.0.1"
	DBRemark  = "debug"
)

// NewVersionCore ...
func NewVersionCore(db *gorm.DB) version.Core {
	vdb := versiondb.NewDB(db)
	core := version.NewCore(vdb)
	isOK := core.IsAutoMigrate(DBVersion)
	vdb.AutoMigrate(isOK)
	if isOK {
		slog.Info("更新数据库表结构")
	}
	orm.SetEnabledAutoMigrate(isOK)
	return core
}
