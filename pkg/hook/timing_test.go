package hook

import (
	"log/slog"
	"testing"
	"time"
)

func TestUseTiming(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	func() {
		cost := UseTiming(time.Second)
		defer cost()
		time.Sleep(200 * time.Millisecond)
	}()

	func() {
		cost := UseTiming(time.Second)
		defer cost()
		time.Sleep(2000 * time.Millisecond)
	}()
}

func TestMD5(t *testing.T) {
	if r := MD5("asbd123"); r != "219262006d1bdd38c740757b30e2a4e8" {
		t.Fatal("expect 219262006d1bdd38c740757b30e2a4e8\ngot ", r)
	}
}
