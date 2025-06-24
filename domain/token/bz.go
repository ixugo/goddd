package token

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

// DelayToken 延迟 token，短期内只会延迟一次，expire 过期时间应该大于 10 分钟
func (c Core) DelayToken(ctx context.Context, token string, expire time.Time) error {
	_, exist := c.data.LoadOrStore(token, struct{}{}, 10*time.Minute)
	if exist {
		return nil
	}
	return c.DelayTokenNow(ctx, token, expire)
}

// DelayTokenNow 立即延迟 token 过期时间
func (c Core) DelayTokenNow(ctx context.Context, token string, expire time.Time) error {
	hash := sha256.Sum256([]byte(token))
	var to Token
	return c.store.Token().Edit(ctx, &to, func(t *Token) {
		t.ExpiredAt.Time = expire
	}, orm.Where("hash = ?", hash[:]))
}

// DelExpired 删除过期的 token
func (c Core) DelExpired(ctx context.Context, before time.Time) ([]string, error) {
	return c.store.Token().DelExpired(ctx, before)
}

// Valid 验证 token 是否过期
func (c Core) Valid(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	var to Token
	to.Hash = hash[:]
	if err := c.store.Token().Get(ctx, &to, orm.Where("hash = ?", hash[:])); err != nil {
		if orm.IsErrRecordNotFound(err) {
			return reason.ErrUnauthorizedToken.SetMsg("请重新登录")
		}
		return reason.ErrDB.Withf("token get err[%s]", err.Error())
	}
	if to.ExpiredAt.Before(time.Now()) {
		if to.Reason != "" {
			return reason.ErrUnauthorizedToken.SetMsg(to.Reason)
		}
		return reason.ErrUnauthorizedToken.SetMsg("请重新登录")
	}
	return nil
}

// DelAllForUser 删除用户的所有 token
func (c Core) DelAllForUser(ctx context.Context, scope, userID string) ([]string, error) {
	return c.store.Token().DelAllForUser(ctx, scope, userID)
}

// Expire 主动过期
func (c Core) Expire(ctx context.Context, scope string, userID string, reason string) ([]string, error) {
	return c.store.Token().Expire(ctx, scope, userID, reason)
}
