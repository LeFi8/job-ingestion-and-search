//go:build integration

package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/lefi8/job-ingestion-and-search/backend/migrations"
)

func Start(ctx context.Context) (string, func(), error) {
	container, err := tcpostgres.Run(ctx, "postgres:18.4", tcpostgres.BasicWaitStrategies())
	if err != nil {
		return "", nil, fmt.Errorf("start postgres container: %w", err)
	}
	stop := func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			slog.Error("terminate postgres container", "error", err)
		}
	}

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		stop()
		return "", nil, fmt.Errorf("container connection string: %w", err)
	}
	if err := migrate(ctx, url); err != nil {
		stop()
		return "", nil, fmt.Errorf("migrate test database: %w", err)
	}

	return url, stop, nil
}

func migrate(ctx context.Context, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
