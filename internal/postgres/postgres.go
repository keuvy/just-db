package postgres

import (
	"context"
	"fmt"
	"io"

	"just-db/internal/engine"
	"just-db/internal/tools"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) Name() engine.Name   { return engine.Postgres }
func (e *Engine) DisplayName() string { return "PostgreSQL" }
func (e *Engine) DefaultPort() int    { return 5432 }

func (e *Engine) DetectTools(ctx context.Context) (engine.Tools, error) {
	return engine.Tools{
		Dump:    tools.Detect(ctx, "pg_dump"),
		Restore: tools.Detect(ctx, "pg_restore"),
		Client:  tools.Detect(ctx, "psql"),
	}, nil
}

func (e *Engine) TestConnection(_ context.Context, _ engine.Connection) error {
	return fmt.Errorf("%w: postgres test-connection", engine.ErrNotImplemented)
}

func (e *Engine) Export(_ context.Context, _ engine.Connection, _ engine.ExportOptions, _ io.Writer) error {
	return fmt.Errorf("%w: postgres export", engine.ErrNotImplemented)
}

func (e *Engine) Import(_ context.Context, _ engine.Connection, _ engine.ImportOptions, _ io.Reader) error {
	return fmt.Errorf("%w: postgres import", engine.ErrNotImplemented)
}
