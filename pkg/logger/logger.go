package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/DeRuina/timberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// NewJSONLogger 创建JSON日志
func NewJSONLogger(debug bool, w io.Writer, sampler Sampler) *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	config.NameKey = ""
	mulitWriteSyncer := []zapcore.WriteSyncer{
		zapcore.AddSync(w),
	}
	if debug {
		mulitWriteSyncer = append(mulitWriteSyncer, zapcore.AddSync(os.Stdout))
	}
	core := zapcore.NewSamplerWithOptions(zapcore.NewCore(
		zapcore.NewJSONEncoder(config),
		zapcore.NewMultiWriteSyncer(mulitWriteSyncer...),
		Level,
	), time.Duration(sampler.TickSec)*time.Second, sampler.First, sampler.Thereafter)
	return zap.New(core, zap.AddCaller())
}

// newRotateWriter 创建日志轮转写入器，支持大小+时间双重轮转，启动时自动清理过期日志
func newRotateWriter(cfg FileConfig) *timberjack.Logger {
	cfg = cfg.ensureNonZero()

	return &timberjack.Logger{
		Filename:         filepath.Join(cfg.Dir, cfg.Name),
		MaxSize:          cfg.MaxSize,
		MaxAge:           cfg.MaxAge,
		MaxBackups:       cfg.MaxBackups,
		Compression:      "gzip",
		RotationInterval: cfg.RotationTime,
		FileMode:         0o644,
		Compress:         cfg.Compress,
	}
}

// SetupSlog 初始化日志，建议使用 NewDefaultConfig() 创建配置
func SetupSlog(cfg Config) (*slog.Logger, func()) {
	SetLevel(cfg.Level)

	sampler := cfg.Sampler.ensureNonZero()

	r := newRotateWriter(cfg.FileConfig)
	log := slog.New(
		newSlog(
			NewJSONLogger(cfg.Debug, r, sampler).Core(),
			zapslog.WithCaller(cfg.Debug),
		),
	)

	if cfg.ServiceID != "" {
		log = log.With("service_id", cfg.ServiceID)
	}
	if cfg.ServiceName != "" {
		log = log.With("service_name", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "" {
		log = log.With("service_version", cfg.ServiceVersion)
	}
	slog.SetDefault(log)

	file, err := os.OpenFile(filepath.Join(cfg.Dir, "crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		info, err := file.Stat()
		if err == nil && info.Size() > 1 {
			_, _ = fmt.Fprintf(file, "\n\n>>> %s\n\n", time.Now())
		}

		_ = SetCrashOutput(file)
	}
	return log, func() {
		if err := r.Close(); err != nil {
			fmt.Println("关闭日志文件失败", err)
		}
		if file != nil {
			if err := file.Close(); err != nil {
				fmt.Println("关闭崩溃日志文件失败", err)
			}
		}
	}
}

// SetCrashOutput recover panic
func SetCrashOutput(f *os.File) error {
	return debug.SetCrashOutput(f, debug.CrashOptions{})
}
