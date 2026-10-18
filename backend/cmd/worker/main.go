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
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage"
)

const (
	purgeInterval = time.Hour
	purgeAge      = 7 * 24 * time.Hour
)

func main() {
	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, cfg)
	stop()
	if err != nil {
		slog.Error("worker failed", "error", err)
		os.Exit(1)
	}
	slog.Info("worker stopped")
}

func run(ctx context.Context, cfg config.Worker) error {
	log := slog.Default()

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := storage.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	es, err := search.New(cfg.ElasticsearchURL)
	if err != nil {
		return err
	}
	if err := es.EnsureIndex(connectCtx); err != nil {
		return fmt.Errorf("ensure index: %w", err)
	}

	go purgeLoop(ctx, store, log)

	relay := &outbox.Relay{Store: store, Index: es, Batch: cfg.BatchSize, Poll: cfg.PollInterval, Log: log}
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
