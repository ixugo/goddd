package orm

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ logger.Interface = (*Logger)(nil)

type Config struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	SlowThreshold   time.Duration
}

type GormOption func(*gorm.Config)

// WithGormLogger 如果需要自定义 logger 的创建，仅供参考
func WithGormLogger(l *slog.Logger, slow time.Duration) GormOption {
	return func(c *gorm.Config) {
		c.Logger = NewLogger(l, slow)
	}
}

// New ...
// 默认采用 slog.Default() 记录日志，如果日志是 debug 级别会输出所有 sql
// warn 级别用于记录慢 sql
func New(dialector gorm.Dialector, cfg Config, opts ...GormOption) (*gorm.DB, error) {
	c := gorm.Config{
		Logger:         NewLogger(slog.Default(), cfg.SlowThreshold),
		TranslateError: true,
	}
	for i := range opts {
		opts[i](&c)
	}
	db, err := gorm.Open(dialector, &c)
	if err != nil {
		return nil, err
	}

	// 检查连接状态
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, err
	}
	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return db, nil
}

type Engine struct {
	db *gorm.DB
}

func NewEngine(db *gorm.DB) Engine {
	return Engine{
		db: db,
	}
}

var (
	ErrRecordNotFound = gorm.ErrRecordNotFound
	ErrDuplicatedKey  = gorm.ErrDuplicatedKey
)

func IsErrRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}

func IsDuplicatedKey(err error) bool {
	return errors.Is(err, ErrDuplicatedKey)
}

func (e Engine) InsertOne(model Tabler) error {
	return e.db.Create(model).Error
}

type Option func(*gorm.DB)

func (e Engine) DeleteOne(model Tabler, opts ...Option) error {
	db := e.db.Model(model)
	if len(opts) == 0 {
		return fmt.Errorf("没有指定删除参数")
	}
	for i := range opts {
		opts[i](db)
	}
	return db.Delete(model).Error
}

func (e Engine) UpdateOne(model Tabler, id int, data map[string]any) error {
	db := e.db.Model(model)
	WithID(id)(db)
	err := db.Updates(data).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicatedKey
	}
	return err
}

// FirstOrCreate true:创建;false:查询
func (e Engine) FirstOrCreate(b any) (bool, error) {
	tx := e.db.FirstOrCreate(b)
	return tx.RowsAffected == 1, tx.Error
}

func (e Engine) Find(model Tabler, bean any, opts ...Option) (total int64, err error) {
	db := e.db.Model(model)
	for i := range opts {
		opts[i](db)
	}
	err = db.Scan(bean).Limit(-1).Offset(-1).Count(&total).Error
	return
}

// NextSeq 获取序列下一个值
func (e Engine) NextSeq(model Tabler) (nextID int, err error) {
	db := e.db.Model(model)
	err = db.Raw(fmt.Sprintf(`SELECT nextval('%s_id_seq'::regclass)`, model.TableName())).Scan(&nextID).Error
	return
}

// WithID ...
func WithID(id int) Option {
	return func(d *gorm.DB) {
		d.Where("id=?", id)
	}
}

func WithLimit(limit, offset int) Option {
	return func(d *gorm.DB) {
		if limit > 0 {
			d.Limit(limit)
		}
		if offset > 0 {
			d.Offset(offset)
		}
	}
}

func WithCreatedAt(startAt, endAt int64) Option {
	return func(d *gorm.DB) {
		if startAt > 0 {
			start := time.Unix(startAt, 0)
			d.Where("created_at >= ?", start.Format(time.DateTime))
		}
		if endAt > 0 {
			end := time.Unix(endAt, 0)
			d.Where("created_at < ?", end.Format(time.DateTime))
		}
	}
}

func GenerateRandomString(length int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	lettersLength := big.NewInt(int64(len(letterBytes)))
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		idx, _ := rand.Int(rand.Reader, lettersLength)
		result[i] = letterBytes[idx.Int64()]
	}
	return string(result)
}
