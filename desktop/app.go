package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
	"just-db/internal/job"
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

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
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
	return a.reg.Infos(a.context())
}

func (a *App) TestConnection(name string, cfg engine.Connection) error {
	eng, err := a.reg.Get(engine.Name(name))
	if err != nil {
		return err
	}
	return eng.TestConnection(a.context(), cfg)
}

func (a *App) Export(name string, cfg engine.Connection, opts engine.ExportOptions, destPath string) (engine.ExportResult, error) {
	eng, err := a.reg.Get(engine.Name(name))
	if err != nil {
		return engine.ExportResult{}, err
	}
	return job.ExportToFile(a.context(), eng, cfg, opts, destPath)
}

func (a *App) ImportDump(name string, cfg engine.Connection, opts engine.ImportOptions, srcPath string) error {
	eng, err := a.reg.Get(engine.Name(name))
	if err != nil {
		return err
	}
	return job.ImportFromFile(a.context(), eng, cfg, opts, srcPath)
}

func (a *App) ListDumps(dir string) ([]engine.DumpFile, error) {
	if dir == "" {
		return []engine.DumpFile{}, nil
	}
	return job.ListDumps(dir)
}

func (a *App) PickSavePath(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{
		Title:           "Save dump",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "PostgreSQL dump", Pattern: "*.dump;*.sql"},
		},
	})
}

func (a *App) PickOpenPath() (string, error) {
	return runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
		Title: "Open dump",
		Filters: []runtime.FileFilter{
			{DisplayName: "PostgreSQL dump", Pattern: "*.dump;*.sql"},
		},
	})
}
