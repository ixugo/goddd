# Store 缓存层规范

godddx 生成的 `store/<domain>cache/` 默认使用 `conc.Cacher`（进程内内存 TTL 缓存）。当需要 **Redis 缓存**（多副本共享、长 TTL、高频读场景）时，按本文档规范改造。

## 判断缓存类型

修改 `store/<domain>cache/` 时，**首先**判断是内存缓存还是 Redis 缓存：

| 类型 | 依赖 | 适用场景 |
|------|------|---------|
| 内存缓存 | `conc.Cacher` (`conc.NewTTLCache`) | 单副本、短 TTL、数据量小 |
| Redis 缓存 | `*redis.Client` (`github.com/redis/go-redis/v9`) | 多副本共享、长 TTL、高频读 |

**如果是 Redis 缓存**：删除 `conc.Cacher` 依赖，替换为 `*redis.Client`。

## 防竞态缓存操作（核心规则）

### 问题

简单的 DEL 后回填会导致竞态：

```
T1: GET miss → 查 DB 得 v1
T2: UPDATE → DB 写入 v2 → DEL 缓存
T1: SET 缓存 v1 （脏数据！DEL 已经执行完了）
```

### 解决方案

| 操作 | Redis 命令 | 理由 |
|------|-----------|------|
| 读穿透回填 | `singleflight.Do` + `SetNX` | singleflight 合并并发穿透 + SetNX 不覆盖写入的新值 |
| Create | `Set(ctx, key, val, ttl)` | 新记录直接写入缓存（`SetEx` 已弃用，用 `Set` + TTL 替代） |
| Update | `Set(ctx, key, val, ttl)` | 写完 DB 后用最新值覆盖缓存，防止读穿透回填旧值 |
| Delete | `Expire(key, 3s)` | 墓碑保护期 3s，覆盖 DB 查询+序列化+网络抖动的最大时延，防 SetNX 回填已删记录 |
| WarmUp | `SetNX` | 不覆盖运行期间已更新的缓存 |

## Redis 缓存改造步骤

### 1. 改造 cache.go

```go
package xxxcache

import (
    "gdylzh.com/elink-gokit/internal/core/xxx"
    "github.com/redis/go-redis/v9"
    "golang.org/x/sync/singleflight"
)

var _ xxx.Storer = (*Cache)(nil)

func NewCache(store xxx.Storer, rdb *redis.Client) *Cache {
    return &Cache{store: store, rdb: rdb}
}

type Cache struct {
    store xxx.Storer
    rdb   *redis.Client
    sf    singleflight.Group // 防缓存击穿：同一 key 并发穿透合并为一次 DB 查询
}

func (c *Cache) Entity() xxx.EntityStorer {
    return (*Entity)(c)
}
```

### 2. 实现实体缓存方法

```go
package xxxcache

import (
    "context"
    "encoding/json"
    "time"

    "gdylzh.com/elink-gokit/internal/core/xxx"
    "gdylzh.com/elink-gokit/pkg/orm"
    "gorm.io/gorm"
)

const (
    keyPrefix = "xxx:key:"
    keyTTL    = 24 * time.Hour
)

type Entity Cache

func (c *Entity) cacheKey(key string) string {
    return keyPrefix + key
}

// GetByKey 按业务键查 Redis，miss 时通过 singleflight 合并并发穿透，用 SETNX 回填。
func (c *Entity) GetByKey(ctx context.Context, key string) (*xxx.Entity, error) {
    cacheKey := c.cacheKey(key)
    data, err := c.rdb.Get(ctx, cacheKey).Bytes()
    if err == nil {
        var out xxx.Entity
        if json.Unmarshal(data, &out) == nil {
            return &out, nil
        }
    }
    v, err, _ := (*Cache)(c).sf.Do(key, func() (any, error) {
        out, err := c.store.Entity().GetByKey(ctx, key)
        if err != nil {
            return nil, err
        }
        if b, _ := json.Marshal(out); b != nil {
            c.rdb.SetNX(ctx, cacheKey, b, keyTTL)
        }
        return out, nil
    })
    if err != nil {
        return nil, err
    }
    return v.(*xxx.Entity), nil
}

// Create 写完 DB 后用 SETEX 写入缓存。
func (c *Entity) Create(ctx context.Context, model *xxx.Entity) error {
    if err := c.store.Entity().Create(ctx, model); err != nil {
        return err
    }
    c.setCache(ctx, model)
    return nil
}

// Update 写完 DB 后用 SETEX 覆盖缓存为最新值。
func (c *Entity) Update(ctx context.Context, model *xxx.Entity, changeFn func(*xxx.Entity), opts ...orm.QueryOption) error {
    if err := c.store.Entity().Update(ctx, model, changeFn, opts...); err != nil {
        return err
    }
    c.setCache(ctx, model)
    return nil
}

// Delete 写完 DB 后将缓存 TTL 缩至 3s（墓碑保护期），防止并发 SetNX 回填已删记录。
func (c *Entity) Delete(ctx context.Context, model *xxx.Entity, opts ...orm.QueryOption) error {
    if err := c.store.Entity().Delete(ctx, model, opts...); err != nil {
        return err
    }
    c.rdb.Expire(ctx, c.cacheKey(model.Key), 3*time.Second)
    return nil
}

// setCache 将实体序列化后写入 Redis，附带 TTL。
func (c *Entity) setCache(ctx context.Context, model *xxx.Entity) {
    if b, err := json.Marshal(model); err == nil {
        c.rdb.Set(ctx, c.cacheKey(model.Key), b, keyTTL)
    }
}

// 不走缓存的方法直接透传
func (c *Entity) List(ctx context.Context, ...) (int64, error) {
    return c.store.Entity().List(ctx, ...)
}
func (c *Entity) Get(ctx context.Context, model *xxx.Entity, opts ...orm.QueryOption) error {
    return c.store.Entity().Get(ctx, model, opts...)
}
func (c *Entity) Count(ctx context.Context, opts ...orm.QueryOption) (int64, error) {
    return c.store.Entity().Count(ctx, opts...)
}
```

### 3. 实现 WarmUp

```go
// WarmUp 启动时预热：全量加载写入 Redis，用 SETNX 不覆盖已有缓存。
func (c *Cache) WarmUp(ctx context.Context) {
    pager := web.NewPagerFilterMaxSize()
    var items []*xxx.Entity
    _, err := c.store.Entity().List(ctx, &items, pager)
    if err != nil {
        slog.ErrorContext(ctx, "xxx cache WarmUp failed", "err", err)
        return
    }
    count := 0
    for _, item := range items {
        data, _ := json.Marshal(item)
        c.rdb.SetNX(ctx, keyPrefix+item.Key, data, keyTTL)
        count++
    }
    slog.InfoContext(ctx, "xxx cache WarmUp done", "count", count)
}
```

### 4. API 层装配

```go
func NewXxxCore(db *gorm.DB, rdb *redis.Client) xxx.Core {
    dbStore := xxxdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    store := xxxcache.NewCache(dbStore, rdb)
    store.WarmUp(context.Background())
    return xxx.NewCore(store)
}
```

## Key 命名规范

- 前缀统一小写，用冒号分隔：`domain:dimension:value`
- 示例：`app:ak:abc123`、`greet:openapi`（Hash key）
- 维度名用缩写：`ak` = access_key，`id` = primary key

## 要点

- WarmUp 在 `NewXxxCore` 中调用，Redis 不可达时仅打日志不阻塞启动
- 不走缓存的方法（List、Count、通用 Get）直接透传到 DB 层
- TTL 按业务需要设置，长期不变的数据可设 365 天
- 多副本部署下 Update 用 `Set` + TTL 覆盖（而非 DEL），确保最终一致
- **singleflight 防击穿**：Cache 结构体持有 `singleflight.Group`，读穿透时用 `sf.Do(key, fn)` 包裹 DB 查询 + SetNX，同一 key 并发穿透只查一次 DB
