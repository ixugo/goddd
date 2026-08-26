package versiondb

import (
	"errors"
	"testing"

	"github.com/ixugo/goddd/domain/version"
	"github.com/ixugo/goddd/pkg/testx"
	"gorm.io/gorm"
)

// TestFirstWithoutTable 验证 versions 表不存在时 First 返回 ErrRecordNotFound,
// 启动流程依赖此行为跳过首次运行的版本检查
func TestFirstWithoutTable(t *testing.T) {
	t.Parallel()
	db := testx.NewDB(t)

	var v version.Version
	err := NewDB(db).First(&v)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("期望 ErrRecordNotFound, got %v", err)
	}
}

// TestAddAndFirst 验证 First 按 id 倒序返回最新一条版本记录
func TestAddAndFirst(t *testing.T) {
	t.Parallel()
	db := testx.NewDB(t, new(version.Version))
	store := NewDB(db)

	for _, ver := range []string{"v1.0.0", "v1.1.0"} {
		if err := store.Add(&version.Version{Version: ver, Remark: "test"}); err != nil {
			t.Fatalf("Add 失败: %v", err)
		}
	}

	var v version.Version
	if err := store.First(&v); err != nil {
		t.Fatalf("First 失败: %v", err)
	}
	if v.Version != "v1.1.0" {
		t.Fatalf("期望最新版本 v1.1.0, got %s", v.Version)
	}
}
