package logger

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestSlog(t *testing.T) {
	log, _ := SetupSlog(Config{
		Debug: true,
	})

	ctx := WithAttr(context.Background(), slog.String("key", "value"))
	log.InfoContext(ctx, "Hello World")
}

func TestLog(t *testing.T) {
	SetupSlog(Config{
		Dir:            "./log",
		ServiceID:      "test",
		ServiceName:    "test",
		ServiceVersion: "1.0.0",
		Debug:          true,
		MaxAge:         7 * 24 * time.Hour,
	})
	os.MkdirAll("./log", 0o755)
	for range 10 {
		go SetupSlog(Config{
			Dir:            "./log",
			ServiceID:      "test",
			ServiceName:    "test",
			ServiceVersion: "1.0.0",
			Debug:          true,
			MaxAge:         time.Second,
		})
		slog.Info("test")
	}

	time.Sleep(5 * time.Second)
}
