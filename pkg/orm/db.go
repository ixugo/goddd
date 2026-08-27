package orm

import (
	"context"
	"crypto/rand"
	"errors"
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
	ConnMaxIdleTime time.Duration
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
		Logger:                 NewLogger(slog.Default(), cfg.SlowThreshold),
		TranslateError:         true,
		SkipDefaultTransaction: true,
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
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	return db, nil
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

func GenerateRandomString(length int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	lettersLength := big.NewInt(int64(len(letterBytes)))
	result := make([]byte, length)
	for i := range length {
		idx, _ := rand.Int(rand.Reader, lettersLength)
		result[i] = letterBytes[idx.Int64()]
	}
	return string(result)
}
