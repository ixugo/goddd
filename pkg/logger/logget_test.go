package logger

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestSlog(t *testing.T) {
	dir := t.TempDir()
	log, cleanup := SetupSlog(Config{
		Dir:   dir,
		Debug: true,
	})
	defer cleanup()

	ctx := WithAttr(context.Background(), slog.String("key", "value"))
	log.InfoContext(ctx, "Hello World")
}

func TestLog(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir, 0o755)

	_, cleanup := SetupSlog(Config{
		Dir:            dir,
		ServiceID:      "test",
		ServiceName:    "test",
		ServiceVersion: "1.0.0",
		Debug:          true,
		MaxAge:         7 * 24 * time.Hour,
	})
	defer cleanup()

	for range 10 {
		slog.Info("test")
	}
}

func TestRotation(t *testing.T) {
	log, cleanup := SetupSlog(Config{
		Dir:            "./logs",
		ServiceID:      "test",
		ServiceName:    "test",
		ServiceVersion: "1.0.0",
		Debug:          false,
		MaxAge:         24 * time.Hour,
		RotationTime:   5 * time.Second,
		RotationSize:   100,
	})
	defer cleanup()

	for range 30 {
		log.Info("test rotation message with enough content to trigger size-based rotation")
		time.Sleep(200 * time.Millisecond)
	}
}
