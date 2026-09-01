# Store 缓存层规范

goddd 生成的 `stores/<domain>cache/` 默认使用 `conc.Cacher`（进程内内存 TTL 缓存）。当需要 **Redis 缓存**（多副本共享、长 TTL、高频读场景）时，按本文档规范改造。

---

## 1. 判断缓存类型

修改 `stores/<domain>cache/` 时，**首先**判断是内存缓存还是 Redis 缓存：

| 类型 | 依赖 | 适用场景 |
|------|------|---------|
| 内存缓存 | `conc.Cacher` (`conc.NewTTLCache`) | 单副本、短 TTL、数据量小 |
| Redis 缓存 | `redis.Cmdable` (`github.com/redis/go-redis/v9`) | 多副本共享、长 TTL、高频读 |

**如果是 Redis 缓存**：删除 `conc.Cacher` 依赖，替换为 `redis.Cmdable`（接口类型，兼容 `*redis.Client` 单机和 `*redis.ClusterClient` 集群）。

---

## 2. 防竞态缓存操作（核心规则）

### 竞态问题

简单的 DEL 后回填会导致并发脏数据：

```
T1: GET miss → 查 DB 得 v1
T2: UPDATE → DB 写入 v2 → DEL 缓存
T1: SET 缓存 v1 （脏数据！DEL 已经执行完毕）
```

### 解决方案

| 操作 | Redis 命令 | 理由 |
|------|-----------|------|
| 读穿透回填 | `singleflight.Do` + `SetNX` | singleflight 合并并发穿透 + SetNX 不覆盖写入的新值 |
| Create | 不写缓存 | 新记录等首次读取时由 SetNX 回填 |
| Update | `Set(ctx, key, val, ttl)` | 写完 DB 后用最新值覆盖缓存，防止读穿透回填旧值 |
| WarmUp | `SetNX` | 不覆盖运行期间已更新的缓存 |

---

## 3. Redis 缓存改造步骤

### 1. 改造 cache.go

```go
package xxxcache

import (
    "github.com/ixugo/goddd/internal/core/xxx"
    "github.com/redis/go-redis/v9"
    "golang.org/x/sync/singleflight"
)

var _ xxx.Storer = (*Cache)(nil)

func NewCache(store xxx.Storer, rdb redis.Cmdable) *Cache {
    c := &Cache{store: store, rdb: rdb}
    // 子 storer 于构造时预建，访问器直返字段，热路径零分配
    c.entity = &Entity{store: store.Entity(), rdb: rdb, sf: &c.sf}
    return c
}

type Cache struct {
    store  xxx.Storer
    entity xxx.EntityStorer
    rdb    redis.Cmdable
    sf     singleflight.Group // 防缓存击穿：同一 key 并发穿透合并为一次 DB 查询
}

func (c *Cache) Entity() xxx.EntityStorer {
    return c.entity
}

func (c *Cache) Begin() (orm.Tx, error) {
    return c.store.Begin()
}
```

### 2. 实现实体缓存方法

```go
package xxxcache

import (
    "context"
    "encoding/json"
    "time"

    "github.com/ixugo/goddd/internal/core/xxx"
    "github.com/ixugo/goddd/pkg/orm"
    "github.com/redis/go-redis/v9"
    "golang.org/x/sync/singleflight"
)

const (
    keyPrefix = "xxx:key:"
    keyTTL    = 24 * time.Hour
)

// Entity 缓存层，写操作同步维护缓存。
// inTx 标记事务副本：事务可能回滚，副本内写操作只做缓存失效、读操作直连 db，
// 避免缓存残留未提交的数据。
type Entity struct {
    store xxx.EntityStorer
    rdb   redis.Cmdable
    sf    *singleflight.Group
    inTx  bool
}

func (c *Entity) cacheKey(key string) string {
    return keyPrefix + key
}

// GetByKey 按业务键查 Redis，miss 时通过 singleflight 合并并发穿透，用 SETNX 回填。
// 事务副本内直连 db，保证读到事务内的最新数据。
func (c *Entity) GetByKey(ctx context.Context, key string) (*xxx.Entity, error) {
    if c.inTx {
        return c.store.GetByKey(ctx, key)
    }
    cacheKey := c.cacheKey(key)
    data, err := c.rdb.Get(ctx, cacheKey).Bytes()
    if err == nil {
        var out xxx.Entity
        if json.Unmarshal(data, &out) == nil {
            return &out, nil
        }
    }
    v, err, _ := c.sf.Do(key, func() (any, error) {
        out, err := c.store.GetByKey(ctx, key)
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

// Create 只写 DB，不写缓存，等首次读取时由 SetNX 回填。
func (c *Entity) Create(ctx context.Context, model *xxx.Entity) error {
    return c.store.Create(ctx, model)
}

// Update 写完 DB 后用最新值覆盖缓存；事务副本内仅删除失效，回滚不残留脏缓存。
func (c *Entity) Update(ctx context.Context, model *xxx.Entity, changeFn func(*xxx.Entity) error) error {
    if err := c.store.Update(ctx, model, changeFn); err != nil {
        return err
    }
    if c.inTx {
        c.rdb.Del(ctx, c.cacheKey(model.Key))
        return nil
    }
    c.setCache(ctx, model)
    return nil
}

// setCache 将实体序列化后写入 Redis，附带 TTL。
func (c *Entity) setCache(ctx context.Context, model *xxx.Entity) {
    if b, err := json.Marshal(model); err == nil {
        c.rdb.Set(ctx, c.cacheKey(model.Key), b, keyTTL)
    }
}

// GetByID 按主键查询，走缓存；事务副本内直连 db，保证读到事务内的最新数据。
func (c *Entity) GetByID(ctx context.Context, id int64) (*xxx.Entity, error) {
    if c.inTx {
        return c.store.GetByID(ctx, id)
    }
    cacheKey := c.cacheKey(fmt.Sprintf("%d", id))
    data, err := c.rdb.Get(ctx, cacheKey).Bytes()
    if err == nil {
        var out xxx.Entity
        if json.Unmarshal(data, &out) == nil {
            return &out, nil
        }
    }
    v, err, _ := c.sf.Do(fmt.Sprintf("%d", id), func() (any, error) {
        out, err := c.store.GetByID(ctx, id)
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

// WithTx 返回保留缓存封装的事务副本：事务内写操作仅失效缓存、读操作直连 db。
func (c *Entity) WithTx(tx orm.Tx) xxx.EntityStorer {
    return &Entity{store: c.store.WithTx(tx), rdb: c.rdb, sf: c.sf, inTx: true}
}

// Delete 删除后写入墓碑：阻止读穿透的 SetNX 把已删除的旧值回填复活。
func (c *Entity) Delete(ctx context.Context, model *xxx.Entity) error {
    if err := c.store.Delete(ctx, model); err != nil {
        return err
    }
    c.rdb.Set(ctx, c.cacheKey(model.Key), "__tombstone__", keyTTL)
    return nil
}

// 不走缓存的方法直接透传
func (c *Entity) List(ctx context.Context, in *xxx.ListEntityInput) ([]*xxx.Entity, int64, error) {
    return c.store.List(ctx, in)
}
func (c *Entity) Count(ctx context.Context, in *xxx.ListEntityInput) (int64, error) {
    return c.store.Count(ctx, in)
}
```

### 3. 实现 WarmUp

```go
// WarmUp 启动时预热：全量加载写入 Redis，用 SETNX 不覆盖已有缓存。
func (c *Cache) WarmUp(ctx context.Context) {
    pager := web.NewPagerFilterMaxSize()
    in := &xxx.ListEntityInput{PagerFilter: pager}
    items, _, err := c.store.Entity().List(ctx, in)
    if err != nil {
        slog.ErrorContext(ctx, "xxx cache WarmUp failed", "err", err)
        return
    }
    count := 0
    for _, item := range items {
        data, _ := json.Marshal(item)
        c.rdb.SetNX(ctx, c.cacheKey(item.Key), data, keyTTL)
        count++
    }
    slog.InfoContext(ctx, "xxx cache WarmUp done", "count", count)
}
```

### 4. API 层装配

```go
func NewXxxCore(db *gorm.DB, rdb redis.Cmdable) xxx.Core {
    dbStore := xxxdb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
    store := xxxcache.NewCache(dbStore, rdb)
    store.WarmUp(context.Background())
    return xxx.NewCore(store)
}
```

---

## 4. Key 命名规范与穿透防护

- **Key 格式**：`domain:dimension:value`（全部小写、冒号分隔，如 `user:id:1001`）
- **空值防穿透**：当 DB 中数据不存在时，可短时间缓存空标记（如 `null`，TTL 30 秒），防止恶意高频请求打崩 DB。
- **singleflight 防击穿**：并发穿透时通过 `sf.Do(key, fn)` 合并为一次 DB 查询。
- **事务副本语义**：`WithTx` 副本在事务内仅对写操作 Del/写墓碑，读操作直连 DB，回滚不残留脏缓存。
