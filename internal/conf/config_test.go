package conf

import (
	"testing"
)

func TestWriteConfig(t *testing.T) {
	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8080,
		},
	}}, "test.toml"); err != nil {
		t.Fatal(err)
	}

	if err := WriteConfig(Bootstrap{Server: Server{
		HTTP: ServerHTTP{
			Port: 8081,
		},
	}}, "test.toml"); err != nil {
		t.Fatal(err)
	}
}
