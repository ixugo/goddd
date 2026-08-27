package testx

import (
	"database/sql"
	"testing"
)

// openProbe 按 DSN 打开探针连接,随测试结束自动关闭
func openProbe(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("打开探针连接失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestNewPostgres 验证交付的 DSN 可连、可读写
func TestNewPostgres(t *testing.T) {
	t.Parallel()
	db := openProbe(t, NewPostgres(t))

	if err := db.Ping(); err != nil {
		t.Fatalf("DSN 不可连: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE probe (id int)"); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
}

// TestNewPostgresIsolation 验证两次调用产出相互隔离的库:
// 甲库所建之表,乙库不可见
func TestNewPostgresIsolation(t *testing.T) {
	t.Parallel()
	dbA := openProbe(t, NewPostgres(t))
	dbB := openProbe(t, NewPostgres(t))

	if _, err := dbA.Exec("CREATE TABLE probe (id int)"); err != nil {
		t.Fatalf("甲库建表失败: %v", err)
	}
	var n int
	err := dbB.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_name = 'probe'").Scan(&n)
	if err != nil {
		t.Fatalf("乙库查表失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("期望乙库无 probe 表, got %d,隔离失效", n)
	}
}
