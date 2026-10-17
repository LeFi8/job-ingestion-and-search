package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/config"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/outbox"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

const (
	purgeInterval = time.Hour
	purgeAge      = 7 * 24 * time.Hour
	maxBatchSize  = 10000
)

func main() {
	var env config.Env
	dbURL := env.DatabaseURL()
	esURL := env.String("ELASTICSEARCH_URL")
	batch := env.IntBetween("WORKER_BATCH_SIZE", 1, maxBatchSize)
	poll := env.PositiveDuration("WORKER_POLL_INTERVAL")
	if err := env.Err(); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, dbURL, esURL, batch, poll)
	stop()
	if err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
	slog.Info("worker stopped")
}

func run(ctx context.Context, dbURL, esURL string, batch int, poll time.Duration) error {
	log := slog.Default()

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := postgres.Open(connectCtx, dbURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	es, err := search.New(esURL)
	if err != nil {
		return err
	}
	if err := es.EnsureIndex(connectCtx); err != nil {
		return fmt.Errorf("ensure index: %w", err)
	}

	go purgeLoop(ctx, store, log)

	relay := &outbox.Relay{Store: store, Index: es, Batch: batch, Poll: poll, Log: log}
	return relay.Run(ctx)
}

func purgeLoop(ctx context.Context, store outbox.Store, log *slog.Logger) {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		n, err := store.PurgeProcessed(ctx, purgeAge)
		if err != nil {
			if ctx.Err() == nil {
				log.Error("outbox.purge failed", "error", err)
			}
			continue
		}
		log.Info("outbox.purge", "deleted", n)
	}
}
