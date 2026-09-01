package main

import (
	"context"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
	"just-db/internal/registry"
)

type App struct {
	ctx context.Context
	reg *registry.Registry
}

func NewApp() *App {
	return &App{reg: registry.New()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

type Health struct {
	Status  string `json:"status"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Mode    string `json:"mode"`
}

func (a *App) Health() Health {
	return Health{
		Status:  "ok",
		Name:    appmeta.Name,
		Version: appmeta.Version,
		Mode:    "desktop",
	}
}

func (a *App) Engines() ([]engine.Info, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.reg.Infos(ctx)
}
