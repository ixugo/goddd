package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestSlog(t *testing.T) {
	log, _ := SetupSlog(Config{
		Debug: true,
	})

	ctx := WithAttr(context.Background(), slog.String("key", "value"))
	log.InfoContext(ctx, "Hello World")
}
