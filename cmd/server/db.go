package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/config"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func newDatabasePool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, func() error, error) {
	if cfg.IsDatabaseIAMAuthEnabled {
		return postgres.NewIAMAuthPool(ctx, cfg.DatabaseConn, cfg.CloudSQLConnectionName)
	}
	return postgres.NewPasswordAuthPool(ctx, cfg.DatabaseConn)
}
