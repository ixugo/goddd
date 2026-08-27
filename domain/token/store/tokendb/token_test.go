package tokendb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"
	"time"

	"github.com/ixugo/goddd/domain/token"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/testx/postgres"
)

// hashOf 生成测试用令牌哈希,模拟真实 Token 的 SHA-256 存储形式
func hashOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// newToken 构造测试令牌,expired 控制过期时间在过去还是未来
func newToken(scope, userID, seed string, expired bool) token.Token {
	expiredAt := time.Now().Add(time.Hour)
	if expired {
		expiredAt = time.Now().Add(-time.Hour)
	}
	return token.Token{
		UserID:    userID,
		Scope:     scope,
		Hash:      hashOf(seed),
		ExpiredAt: orm.Time{Time: expiredAt},
	}
}

// TestCreateAndGet 验证令牌写入后可按哈希条件取回
func TestCreateAndGet(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).Token()
	ctx := context.Background()

	want := newToken("api", "u1", "token-1", false)
	if err := store.Create(ctx, &want); err != nil {
		t.Fatalf("Create 失败: %v", err)
	}
	if want.ID == 0 {
		t.Fatal("期望自增主键回填, got 0")
	}

	var got token.Token
	if err := store.Get(ctx, &got, orm.Where("hash=?", want.Hash)); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.UserID != "u1" || got.Scope != "api" {
		t.Fatalf("期望 user_id=u1 scope=api, got user_id=%s scope=%s", got.UserID, got.Scope)
	}
}

// TestExpire 验证 Expire 仅处理未过期令牌,并借助 RETURNING 返回受影响哈希。
// RETURNING 子句在 SQLite 上行为与 Postgres 有差异,此测试必须在真库上运行。
func TestExpire(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).Token()
	ctx := context.Background()

	alive := newToken("api", "u1", "token-alive", false)
	dead := newToken("api", "u1", "token-dead", true)
	other := newToken("api", "u2", "token-other", false)
	for _, m := range []*token.Token{&alive, &dead, &other} {
		if err := store.Create(ctx, m); err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}

	hashes, err := store.Expire(ctx, "api", "u1", "logout")
	if err != nil {
		t.Fatalf("Expire 失败: %v", err)
	}
	wantHash := hex.EncodeToString(alive.Hash)
	if len(hashes) != 1 || hashes[0] != wantHash {
		t.Fatalf("期望仅返回存活令牌哈希 %s, got %v", wantHash, hashes)
	}

	var got token.Token
	if err := store.Get(ctx, &got, orm.Where("hash=?", alive.Hash)); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.Reason != "logout" {
		t.Fatalf("期望 reason=logout, got %s", got.Reason)
	}
}

// TestDeleteAllForUser 验证按用户清空令牌且 RETURNING 返回全部哈希
func TestDeleteAllForUser(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).Token()
	ctx := context.Background()

	a := newToken("api", "u1", "token-a", false)
	b := newToken("api", "u1", "token-b", false)
	other := newToken("api", "u2", "token-c", false)
	for _, m := range []*token.Token{&a, &b, &other} {
		if err := store.Create(ctx, m); err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}

	hashes, err := store.DeleteAllForUser(ctx, "api", "u1")
	if err != nil {
		t.Fatalf("DeleteAllForUser 失败: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("期望删除 2 个令牌, got %d", len(hashes))
	}
	if !slices.Contains(hashes, hex.EncodeToString(a.Hash)) || !slices.Contains(hashes, hex.EncodeToString(b.Hash)) {
		t.Fatalf("返回哈希与预期不符: %v", hashes)
	}

	var list []*token.Token
	total, err := store.List(ctx, &list, nil)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total != 1 || list[0].UserID != "u2" {
		t.Fatalf("期望仅剩 u2 的令牌, got total=%d", total)
	}
}

// TestDeleteExpired 验证仅清理过期令牌,存活令牌不受影响
func TestDeleteExpired(t *testing.T) {
	t.Parallel()
	db := postgres.NewDB(t)
	store := NewDB(db).AutoMigrate(true).Token()
	ctx := context.Background()

	dead := newToken("api", "u1", "token-expired", true)
	alive := newToken("api", "u1", "token-valid", false)
	for _, m := range []*token.Token{&dead, &alive} {
		if err := store.Create(ctx, m); err != nil {
			t.Fatalf("Create 失败: %v", err)
		}
	}

	hashes, err := store.DeleteExpired(ctx, time.Now())
	if err != nil {
		t.Fatalf("DeleteExpired 失败: %v", err)
	}
	wantHash := hex.EncodeToString(dead.Hash)
	if len(hashes) != 1 || hashes[0] != wantHash {
		t.Fatalf("期望仅返回过期令牌哈希 %s, got %v", wantHash, hashes)
	}

	var list []*token.Token
	total, err := store.List(ctx, &list, nil)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total != 1 {
		t.Fatalf("期望剩余 1 个令牌, got %d", total)
	}
}
