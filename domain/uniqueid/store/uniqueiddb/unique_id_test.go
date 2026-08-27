package uniqueiddb

import (
	"context"
	"errors"
	"testing"

	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/testx/postgres"
)

// TestCreateAndGet 验证字符串主键的记录可正常写入并按条件取回
func TestCreateAndGet(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).UniqueID()
	ctx := context.Background()

	want := uniqueid.UniqueID{ID: "01JTEST000000000000000001"}
	if err := store.Create(ctx, &want); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}

	var got uniqueid.UniqueID
	if err := store.Get(ctx, &got, orm.Where("id=?", want.ID)); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.ID != want.ID {
		t.Fatalf("期望 id %s, got %s", want.ID, got.ID)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("期望 created_at 由数据库默认值填充, got 零值")
	}
}

// TestDelete 验证删除后记录不可再查得
func TestDelete(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).UniqueID()
	ctx := context.Background()

	m := uniqueid.UniqueID{ID: "01JTEST000000000000000002"}
	if err := store.Create(ctx, &m); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if err := store.Delete(ctx, &m, orm.Where("id=?", m.ID)); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	var got uniqueid.UniqueID
	err := store.Get(ctx, &got, orm.Where("id=?", m.ID))
	if !errors.Is(err, orm.ErrRecordNotFound) {
		t.Fatalf("期望 ErrRecordNotFound, got %v", err)
	}
}

// TestList 验证分页查询返回总数与记录
func TestList(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).UniqueID()
	ctx := context.Background()

	for range 3 {
		m := uniqueid.UniqueID{ID: orm.GenerateRandomString(26)}
		if err := store.Create(ctx, &m); err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}

	var list []*uniqueid.UniqueID
	total, err := store.List(ctx, &list, nil)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("期望 total=3 len=3, got total=%d len=%d", total, len(list))
	}
}
