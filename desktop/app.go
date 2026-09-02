package main

import (
	"context"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"just-db/internal/appdir"
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
	_, _ = appdir.Backups()
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

type Health struct {
	Status     string `json:"status"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Mode       string `json:"mode"`
	DataDir    string `json:"dataDir"`
	BackupsDir string `json:"backupsDir"`
}

func (a *App) Health() Health {
	root, _ := appdir.Root()
	backups, _ := appdir.Backups()
	return Health{
		Status:     "ok",
		Name:       appmeta.Name,
		Version:    appmeta.Version,
		Mode:       "desktop",
		DataDir:    root,
		BackupsDir: backups,
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
	if destPath == "" {
		dir, err := appdir.Backups()
		if err != nil {
			return engine.ExportResult{}, err
		}
		format := engine.CoerceFormat(eng.Name(), opts.Format)
		destPath = filepath.Join(dir, job.DefaultFileName(eng.Name(), cfg.Database, format))
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
		var err error
		dir, err = appdir.Backups()
		if err != nil {
			return nil, err
		}
	}
	return job.ListDumps(dir)
}

func (a *App) DefaultDumpName(name, database, format string) string {
	engName := engine.Name(name)
	return job.DefaultFileName(engName, database, engine.CoerceFormat(engName, format))
}

func (a *App) PickSavePath(defaultName string) (string, error) {
	dir, _ := appdir.Backups()
	return runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{
		Title:            "Save dump",
		DefaultFilename:  defaultName,
		DefaultDirectory: dir,
		Filters: []runtime.FileFilter{
			{DisplayName: "Database dump", Pattern: "*.dump;*.sql"},
		},
	})
}

func (a *App) PickOpenPath() (string, error) {
	dir, _ := appdir.Backups()
	return runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
		Title:            "Open dump",
		DefaultDirectory: dir,
		Filters: []runtime.FileFilter{
			{DisplayName: "Database dump", Pattern: "*.dump;*.sql"},
		},
	})
}

func (a *App) OpenBackupsDir() error {
	dir, err := appdir.Backups()
	if err != nil {
		return err
	}
	switch goruntime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "windows":
		return exec.Command("explorer", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
