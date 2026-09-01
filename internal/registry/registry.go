package registry

import (
	"context"
	"fmt"

	"just-db/internal/engine"
	"just-db/internal/mysql"
	"just-db/internal/postgres"
)

type Registry struct {
	engines []engine.Engine
}

func New() *Registry {
	return &Registry{
		engines: []engine.Engine{
			postgres.New(),
			mysql.New(),
		},
	}
}

func (r *Registry) All() []engine.Engine {
	return append([]engine.Engine(nil), r.engines...)
}

func (r *Registry) Get(name engine.Name) (engine.Engine, error) {
	for _, e := range r.engines {
		if e.Name() == name {
			return e, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", engine.ErrUnknownEngine, name)
}

func (r *Registry) Infos(ctx context.Context) ([]engine.Info, error) {
	infos := make([]engine.Info, 0, len(r.engines))
	for _, e := range r.engines {
		tools, err := e.DetectTools(ctx)
		if err != nil {
			return nil, err
		}
		infos = append(infos, engine.Info{
			Name:        e.Name(),
			DisplayName: e.DisplayName(),
			DefaultPort: e.DefaultPort(),
			Tools:       tools,
			Ready:       tools.Complete(),
		})
	}
	return infos, nil
}
