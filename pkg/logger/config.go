package logger

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

// Level 日志级别
var Level = zap.NewAtomicLevelAt(zap.InfoLevel)

type Config struct {
	ServiceID      string  // 服务 ID(可选)
	ServiceName    string  // 服务名称(可选)
	ServiceVersion string  // 服务版本(可选)
	Debug          bool    // 是否开启 debug，日志会同时写终端和文件
	Level          string  // debug/info/warn/error
	Sampler        Sampler // 采样器，用于控制日志写入频率(可选)
	FileConfig             // 日志文件配置
}

type FileConfig struct {
	Dir          string        // 日志写入目录
	Name         string        // 日志文件名(选填)
	MaxAge       int           // 日志保留时间(天)
	RotationTime time.Duration // 日志分割时间
	MaxSize      int           // 日志文件最大大小(MB)
	Compress     bool          // 是否压缩日志
	MaxBackups   int           // 保留的旧日志归档文件最大数量，超出的自动删除
}

type Sampler struct {
	TickSec    int `command:"时间窗口(秒)"`
	First      int `command:"每个时间窗口内记录的前N条日志"`
	Thereafter int `command:"超过N条后每M条记录一次"`
}

// 确保结构体属性非零值
// 默认保留 7 天日志，12 小时分割一个新的日志文件，50MB 的文件即创建新文件，不压缩日志
func (c FileConfig) ensureNonZero() FileConfig {
	if c.Dir == "" {
		c.Dir = "./logs"
	}
	if c.Name == "" {
		c.Name = "app.log"
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 7
	}
	if c.RotationTime <= 0 {
		c.RotationTime = 12 * time.Hour
	}
	if c.MaxSize <= 0 {
		c.MaxSize = 50
	}
	return c
}

func (s Sampler) ensureNonZero() Sampler {
	if s.TickSec <= 0 {
		s.TickSec = 1
	}
	if s.First <= 0 {
		s.First = 5
	}
	if s.Thereafter <= 0 {
		s.Thereafter = 5
	}
	return s
}

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

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() Config {
	return Config{
		Debug:      true,
		Level:      "debug",
		FileConfig: FileConfig{}.ensureNonZero(),
		Sampler:    Sampler{}.ensureNonZero(),
	}
}

func (c Config) SetMaxAge(days int) Config {
	c.MaxAge = days
	return c
}

func (c Config) SetRotationTime(rotationTime time.Duration) Config {
	c.RotationTime = rotationTime
	return c
}

// SetLevel 设置日志级别 debug/info/warn/error
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
