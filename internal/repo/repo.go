package repo

import (
	"context"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ Q *dbgen.Queries }

func New(pool *pgxpool.Pool) *Repo { return &Repo{Q: dbgen.New(pool)} }

func (r *Repo) GetAllFlags(ctx context.Context, env string) ([]dbgen.Flag, error) {
	return r.Q.GetAllFlags(ctx, env)
}

func (r *Repo) UpsertFlag(ctx context.Context, p dbgen.UpsertFlagParams) error {
    return r.Q.UpsertFlag(ctx, p)
}