package conf

import (
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

func DefaultConfig() Bootstrap {
	return Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{
				Port:      8080,
				Timeout:   Duration(30 * time.Second),
				JwtSecret: orm.GenerateRandomString(32),
				PProf: ServerPPROF{
					Enabled:   true,
					AccessIps: []string{"::1", "127.0.0.1"},
				},
			},
		},
		Data: Data{
			Database: Database{
				Dsn:             "./configs/data.db",
				MaxIdleConns:    10,
				MaxOpenConns:    50,
				ConnMaxLifetime: Duration(6 * time.Hour),
				SlowThreshold:   Duration(200 * time.Millisecond),
			},
		},
		Log: Log{
			Dir:          "./logs",
			Level:        "info",
			MaxAge:       Duration(7 * 24 * time.Hour),
			RotationTime: Duration(8 * time.Hour),
			RotationSize: 50,
		},
	}
}
