package orm

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Logger struct {
	*slog.Logger
	slow     time.Duration
	logLevel slog.Level
}

// NewLogger 封装日志
// 如果需要记录全部日志，开启 slog 的 debug 日志级别即可
// 建议 logLevel 用
func NewLogger(l *slog.Logger, slow time.Duration) *Logger {
	return &Logger{Logger: l, slow: slow, logLevel: slog.LevelDebug}
}

func (l *Logger) SetLevel(level slog.Level) {
	l.logLevel = level
}

// Error implements logger.Interface.
// Subtle: this method shadows the method (*Logger).Error of Logger.Logger.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	if l.logLevel <= slog.LevelError {
		l.Logger.ErrorContext(ctx, msg, args...)
	}
}

// Info implements logger.Interface.
// Subtle: this method shadows the method (*Logger).Info of Logger.Logger.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	if l.logLevel <= slog.LevelInfo {
		l.Logger.InfoContext(ctx, msg, args...)
	}
}

// Warn implements logger.Interface.
// Subtle: this method shadows the method (*Logger).Warn of Logger.Logger.
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	if l.logLevel <= slog.LevelWarn {
		l.Logger.WarnContext(ctx, msg, args...)
	}
}

// LogMode implements logger.Interface.
func (l *Logger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	switch level {
	case logger.Silent, logger.Error:
		newLogger.logLevel = slog.LevelError
	case logger.Warn:
		newLogger.logLevel = slog.LevelWarn
	case logger.Info:
		newLogger.logLevel = slog.LevelInfo
	}
	return &newLogger
}

// Trace implements logger.Interface.
func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	// 在业务里通常应该主动处理 ErrDuplicatedKey 错误，这里应该忽略掉
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, gorm.ErrDuplicatedKey) {
		l.Error(ctx, "gorm error", "err", err, "sql", sql, "rows", rows)
		return
	}

	if elapsed > l.slow && l.slow > 0 {
		l.Warn(ctx, "gorm sql slow", "duration_ms", elapsed.Milliseconds(), "sql", sql, "rows", rows)
		return
	}

	// 仅 debug 状态会打印所有 sql
	l.DebugContext(ctx, "gorm trace", "duration_ms", elapsed.Milliseconds(), "sql", sql, "rows", rows)
}
