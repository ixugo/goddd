package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	// "github.com/go-kratos/kratos/v2/log"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// Level 日志级别
var Level = zap.NewAtomicLevelAt(zap.InfoLevel)

// SetLevel 设置日志级别 debug/warn/error
func SetLevel(l string) {
	switch strings.ToLower(l) {
	case "debug":
		Level.SetLevel(zap.DebugLevel)
	case "warn":
		Level.SetLevel(zap.WarnLevel)
	case "error":
		Level.SetLevel(zap.ErrorLevel)
	default:
		Level.SetLevel(zap.InfoLevel)
	}
}

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
	// level = zap.ErrorLevel
	core := zapcore.NewSamplerWithOptions(zapcore.NewCore(
		zapcore.NewJSONEncoder(config),
		zapcore.NewMultiWriteSyncer(mulitWriteSyncer...),
		Level,
	), time.Duration(sampler.TickSec)*time.Second, sampler.First, sampler.Thereafter)
	return zap.New(core, zap.AddCaller())
}

func rotatelog(dir string, maxAge, duration time.Duration, size int64) *rotatelogs.RotateLogs {
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	if duration <= 0 {
		duration = 12 * time.Hour
	}
	if size <= 0 {
		size = 10 * 1024 * 1024
	}
	r, _ := rotatelogs.New(
		filepath.Join(dir, "%Y%m%d_%H_%M_%S.log"),
		rotatelogs.WithMaxAge(maxAge),
		rotatelogs.WithRotationTime(duration),
		rotatelogs.WithRotationSize(size),
	)
	return r
}

// func TracingValue(key string, lo log.Valuer) slog.Attr {
// 	return slog.Attr{
// 		Key:   key,
// 		Value: slog.AnyValue(ValueFunc(lo)),
// 	}
// }

// type ValueFunc log.Valuer

// // MarshalJSON implements json.Marshaler.
// func (v ValueFunc) MarshalJSON() ([]byte, error) {
// 	data := log.Valuer(v)(context.TODO())
// 	return json.Marshal(data)
// }

// func (vf ValueFunc) Value(ctx context.Context) interface{} {
// 	return log.Valuer(vf)(ctx)
// }

// var _ json.Marshaler = (*ValueFunc)(nil)

// var _ log.Logger = (*Logger)(nil)

// type KLogger struct {
// 	log *slog.Logger
// }

// func NewLogger(slog *slog.Logger) log.Logger {
// 	return &Logger{slog}
// }

// func (l *KLogger) Log(level log.Level, keyvals ...interface{}) error {
// 	keylen := len(keyvals)
// 	if keylen == 0 || keylen%2 != 0 {
// 		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
// 		return nil
// 	}

// 	switch level {
// 	case log.LevelDebug:
// 		l.log.Debug("", keyvals...)
// 	case log.LevelInfo:
// 		l.log.Info("", keyvals...)
// 	case log.LevelWarn:
// 		l.log.Warn("", keyvals...)
// 	case log.LevelError:
// 		l.log.Error("", keyvals...)
// 	case log.LevelFatal:
// 		l.log.Error("", keyvals...)
// 	}
// 	return nil
// }

// Config ....
type Config struct {
	Dir            string        // 日志写入目录
	ServiceID      string        // 服务 ID(可选)
	ServiceName    string        // 服务名称(可选)
	ServiceVersion string        // 服务版本(可选)
	Debug          bool          // 是否开启 debug，日志会同时写终端和文件
	MaxAge         time.Duration // 日志保留时间
	RotationTime   time.Duration // 日志分割时间
	RotationSize   int64         // 日志分割大小，单位字节
	Level          string        // debug/info/warn/error
	Sampler        Sampler       // 采样器，用于控制日志写入频率(可选)
}

type Sampler struct {
	TickSec    int `command:"时间窗口(秒)"`
	First      int `command:"每个时间窗口内记录的前N条日志"`
	Thereafter int `command:"超过N条后每M条记录一次"`
}

// func getLevel(level string) zapcore.Level {
// 	switch strings.ToLower(level) {
// 	case "debug":
// 		return zap.DebugLevel
// 	case "info":
// 		return zapcore.InfoLevel
// 	case "warn":
// 		return zap.WarnLevel
// 	case "error":
// 		return zap.ErrorLevel
// 	default:
// 		return zap.InfoLevel
// 	}
// }

// NewDefaultConfig 创建默认配置
// 默认行为
// - 最大 50MB 的文件即创建新文件
// - 12 小时分割一个新的日志文件
// - 仅保留最近 7 天的文件
func NewDefaultConfig() Config {
	return Config{
		ServiceID:      "",
		Dir:            "./logs",
		ServiceVersion: "v0.0.1",
		Debug:          true,
		MaxAge:         7 * 24 * time.Hour,
		RotationTime:   12 * time.Hour,
		RotationSize:   50 * 1024 * 1024,
		Sampler: Sampler{
			TickSec:    1,
			First:      5,
			Thereafter: 5,
		},
	}
}

// SetRotationMB 按照 MB 分割日志文件，time 为分割时间间隔
func (c Config) SetRotationKB(kb int64, duration time.Duration) Config {
	c.RotationSize = kb * 1024
	c.RotationTime = duration
	return c
}

// SetRotation 注意单位是 b
// Deprecated: 建议使用 SetRotationKB
func (c Config) SetRotation(b int64, duration time.Duration) Config {
	c.RotationSize = b
	c.RotationTime = duration
	return c
}

// SetMaxAge 设置日志保留时间
func (c Config) SetMaxAge(maxAge time.Duration) Config {
	c.MaxAge = maxAge
	return c
}

// SetLevel 设置日志级别 debug/info/error
func (c Config) SetLevel(level string) Config {
	c.Level = level
	return c
}

// SetSampler 设置采样器，用于控制日志写入频率(可选)
func (c Config) SetSampler(sampler Sampler) Config {
	c.Sampler = sampler
	return c
}

// SetDebug 设置是否开启 debug，日志会同时写终端和文件
func (c Config) SetDebug(debug bool) Config {
	c.Debug = debug
	return c
}

// SetDir 设置日志写入目录
func (c Config) SetDir(dir string) Config {
	c.Dir = dir
	return c
}

// SetService 设置服务信息，可选
// - 使用 id 作为服务唯一标识
// - 使用 name 作为服务名称
// - 使用 version 作为服务版本
// id,name,version 空串时，将不记录到日志
func (c Config) SetService(id, name, version string) Config {
	c.ServiceID = id
	c.ServiceName = name
	c.ServiceVersion = version
	return c
}

// SetupSlog 初始化日志，建议使用 NewDefaultConfig() 创建配置
func SetupSlog(cfg Config) (*slog.Logger, func()) {
	SetLevel(cfg.Level)

	if cfg.Sampler.TickSec <= 0 {
		cfg.Sampler.TickSec = 1
		cfg.Sampler.First = 5
		cfg.Sampler.Thereafter = 5
	}

	r := rotatelog(cfg.Dir, cfg.MaxAge, cfg.RotationTime, cfg.RotationSize)
	log := slog.New(
		newSlog(
			NewJSONLogger(cfg.Debug, r, cfg.Sampler).Core(),
			zapslog.WithCaller(cfg.Debug),
		),
	)

	if cfg.ServiceID != "" {
		log = log.With("serviceID", cfg.ServiceID)
	}
	if cfg.ServiceName != "" {
		log = log.With("serviceName", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "" {
		log = log.With("serviceVersion", cfg.ServiceVersion)
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
		if file != nil {
			file.Close()
		}
	}
}

// SetCrashOutput recover panic
func SetCrashOutput(f *os.File) error {
	return debug.SetCrashOutput(f, debug.CrashOptions{})
}
