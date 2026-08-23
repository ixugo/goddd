// Package testx 提供数据库集成测试的基础设施:复用同一个 Postgres 容器,
// 每个测试用例创建独立的随机数据库并以 AutoMigrate 建表,测试结束后自动删库。
package testx

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/testx/docker"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 测试容器与数据库的固定参数,镜像取 alpine 变体以缩短 CI 拉取时间
const (
	pgImage      = "postgres:16-alpine"
	pgName       = "godddtest"
	pgPort       = "5432"
	pgUser       = "postgres"
	pgPassword   = "postgres"
	pingTimeout  = 15 * time.Second
	pingInterval = 200 * time.Millisecond
)

// NewDB 为单个测试准备一套干净的 Postgres 数据库。
// 固定名容器跨测试进程复用(测试结束有意保留容器以加速下次运行),
// 每个测试用例在容器内创建随机名数据库实现隔离,t.Cleanup 负责删库。
// models 传入领域模型指针,内部以 AutoMigrate 完成建表。
func NewDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()

	c, err := docker.StartContainer(pgImage, pgName, pgPort,
		[]string{"-e", "POSTGRES_PASSWORD=" + pgPassword}, nil)
	if err != nil {
		t.Fatalf("启动测试容器失败: %v", err)
	}

	ctx := context.Background()
	master := openSQL(t, c.HostPort, "postgres")
	waitReady(t, master)

	// 随机库名保证并行测试互不干扰
	dbName := randomDBName(8)
	if _, err := master.ExecContext(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("创建测试库失败: %v", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", pgUser, pgPassword, c.HostPort, dbName)
	db, err := orm.New(gormpostgres.New(gormpostgres.Config{DriverName: "pgx", DSN: dsn}), orm.Config{
		MaxIdleConns: 2,
		MaxOpenConns: 4,
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}

	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		if _, err := master.ExecContext(ctx, "DROP DATABASE "+dbName); err != nil {
			t.Logf("删除测试库 %s 失败: %v", dbName, err)
		}
		_ = master.Close()
	})

	return db
}

// StopPostgres 停止并删除 Postgres 测试容器。
// 测试流程无需调用(容器有意保留以加速下次运行,CI runner 为一次性虚机亦无需清理),
// 仅供本地开发欲彻底清理时在代码中调用,亦可直接执行:
// docker stop godddtest && docker rm godddtest
func StopPostgres() error {
	return docker.StopContainer(pgName)
}

// openSQL 以 database/sql 直连指定库,供建库/删库等管理操作使用
func openSQL(t *testing.T, hostPort, dbName string) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", pgUser, pgPassword, hostPort, dbName)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("打开管理连接失败: %v", err)
	}
	return db
}

// waitReady 容器启动不等于 Postgres 就绪,轮询 ping 直至可服务或超时
func waitReady(t *testing.T, db *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(pingTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := db.PingContext(ctx)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 Postgres 就绪超时: %v\n容器日志:\n%s", err, docker.DumpContainerLogs(pgName))
		}
		time.Sleep(pingInterval)
	}
}

// randomDBName 生成 n 位小写字母库名,避免并行测试撞名
func randomDBName(n int) string {
	b := make([]byte, n)
	for i := range b {
		// #nosec G404 -- 仅用于测试库命名,无安全语义,无需加密随机源
		b[i] = byte('a' + rand.IntN(26))
	}
	return string(b)
}
