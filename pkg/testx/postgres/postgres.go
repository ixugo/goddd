// Package postgres 提供基于 gorm 的 Postgres 测试便利封装。
// gorm/pgx 客户端依赖囚于本包;仅需连接信息的调用方请用父包 testx.NewPostgres,
// 不受本包依赖传染。
package postgres

import (
	"testing"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/testx"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDB 在独立的随机名测试库上交付 *gorm.DB。
// 建表不归本函数管:是否迁移、迁移何表属业务决策,由调用方经各自 store 的 AutoMigrate 完成。
// 连接池刻意调小:测试库众多,避免容器内连接耗尽。
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := testx.NewPostgres(t)
	db, err := orm.New(gormpostgres.New(gormpostgres.Config{DriverName: "pgx", DSN: dsn}), orm.Config{
		MaxIdleConns: 2,
		MaxOpenConns: 4,
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}

	// t.Cleanup 后进先出:先关连接,再执行 testx.NewPostgres 注册的删库
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}
