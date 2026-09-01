package mysql

import (
	"context"
	"fmt"
	"io"

	"just-db/internal/engine"
	"just-db/internal/tools"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) Name() engine.Name   { return engine.MySQL }
func (e *Engine) DisplayName() string { return "MySQL / MariaDB" }
func (e *Engine) DefaultPort() int    { return 3306 }

func (e *Engine) DetectTools(ctx context.Context) (engine.Tools, error) {
	client := tools.Detect(ctx, "mysql", "mariadb")
	return engine.Tools{
		Dump:    tools.Detect(ctx, "mysqldump", "mariadb-dump"),
		Restore: client,
		Client:  client,
	}, nil
}

func (e *Engine) TestConnection(_ context.Context, _ engine.Connection) error {
	return fmt.Errorf("%w: mysql test-connection", engine.ErrNotImplemented)
}

func (e *Engine) Export(_ context.Context, _ engine.Connection, _ engine.ExportOptions, _ io.Writer) error {
	return fmt.Errorf("%w: mysql export", engine.ErrNotImplemented)
}

func (e *Engine) Import(_ context.Context, _ engine.Connection, _ engine.ImportOptions, _ io.Reader) error {
	return fmt.Errorf("%w: mysql import", engine.ErrNotImplemented)
}
