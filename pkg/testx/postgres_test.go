package testx

import (
	"testing"
)

// pgProbe 仅供 postgres 基建自测的探针模型
type pgProbe struct {
	ID int `gorm:"primaryKey"`
}

// TestNewDB 验证 NewDB 交付的库可正常建表与读写
func TestNewDB(t *testing.T) {
	t.Parallel()
	db := NewDB(t, new(pgProbe))

	if err := db.Create(&pgProbe{}).Error; err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	var count int64
	if err := db.Model(new(pgProbe)).Count(&count).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("期望 count=1, got %d", count)
	}
}

// TestNewDBIsolation 验证两次 NewDB 产出相互隔离的数据库:
// 甲库所建之表,乙库不可见
func TestNewDBIsolation(t *testing.T) {
	t.Parallel()
	dbA := NewDB(t, new(pgProbe))
	dbB := NewDB(t)

	if !dbA.Migrator().HasTable(new(pgProbe)) {
		t.Fatal("期望甲库存在 pg_probes 表")
	}
	if dbB.Migrator().HasTable(new(pgProbe)) {
		t.Fatal("期望乙库不存在 pg_probes 表,隔离失效")
	}
}
