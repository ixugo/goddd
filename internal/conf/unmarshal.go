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
	tmp := path + ".tmp"
	_ = os.RemoveAll(tmp)

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := toml.NewEncoder(f).SetIndentTables(true).Encode(v); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
