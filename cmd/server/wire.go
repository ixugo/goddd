//go:build wireinject
// +build wireinject

package main

import (
	"log/slog"
	"net/http"

	"github.com/google/wire"
	"github.com/ixugo/goddd/internal/conf"
	"github.com/ixugo/goddd/internal/data"
	"github.com/ixugo/goddd/internal/web/api"
)

func wireApp(bc *conf.Bootstrap, log *slog.Logger) (http.Handler, func(), error) {
	panic(wire.Build(data.ProviderSet, api.ProviderVersionSet, api.ProviderSet))
}
