package conf

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// SetupConfig 从文件读取内容初始化配置
func SetupConfig(v any, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(b, v)
}

// WriteConfig 将配置写回文件
func WriteConfig(v any, path string) error {
	b, err := toml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
