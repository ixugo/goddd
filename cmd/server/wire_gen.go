// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/ixugo/goddd/internal/conf"
	"github.com/ixugo/goddd/internal/data"
	"github.com/ixugo/goddd/internal/web/api"
	"log/slog"
	"net/http"
)

// Injectors from wire.go:

func wireApp(bc *conf.Bootstrap, log *slog.Logger) (http.Handler, func(), error) {
	db, err := data.SetupDB(bc, log)
	if err != nil {
		return nil, nil, err
	}
	core := api.NewVersion(db)
	versionAPI := api.NewVersionAPI(core)
	usecase := &api.Usecase{
		Conf:    bc,
		DB:      db,
		Version: versionAPI,
	}
	handler := api.NewHTTPHandler(usecase)
	return handler, func() {
	}, nil
}
