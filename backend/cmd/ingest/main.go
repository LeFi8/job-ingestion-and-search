package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/config"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/langdetect"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage"
)

func main() {
	source := flag.String("source", "", "source id (defaults to the file base name)")
	reindex := flag.Bool("reindex", false, "enqueue every posting for indexing instead of ingesting a file")
	flag.Parse()
	if *reindex && flag.NArg() != 0 {
		slog.Error("usage: ingest --reindex")
		os.Exit(2)
	}
	if !*reindex && flag.NArg() != 1 {
		slog.Error("usage: ingest [--source=<id>] <file> | ingest --reindex")
		os.Exit(2)
	}

	cfg, err := config.LoadIngest()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if *reindex {
		err := reindexAll(ctx, cfg.DatabaseURL)
		stop()
		if err != nil {
			slog.Error("reindex failed", "error", err)
			os.Exit(1)
		}
		return
	}
	path := flag.Arg(0)
	sourceID := *source
	if sourceID == "" {
		sourceID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	sum, err := run(ctx, cfg.DatabaseURL, path, sourceID)
	stop()
	counts := []any{
		"source", sourceID,
		"approved", sum.Approved,
		"rejected", sum.Rejected,
		"unchanged", sum.Unchanged,
		"quarantined", sum.Quarantined,
		"invalid", sum.Invalid,
		"duplicate", sum.Duplicate,
	}
	if err != nil {
		slog.Error("ingest failed", append(counts, "error", err)...)
		os.Exit(1)
	}
	slog.Info("ingest summary", counts...)
	if sum.Invalid > 0 {
		slog.Error("ingest finished with refused rows", "invalid", sum.Invalid)
		os.Exit(1)
	}
}

func reindexAll(ctx context.Context, dbURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	store, err := storage.Open(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	n, err := store.EnqueueAll(ctx)
	if err != nil {
		return err
	}
	slog.Info("reindex enqueued", "enqueued", n)
	return nil
}

func run(ctx context.Context, dbURL, path, sourceID string) (ingestion.Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return ingestion.Summary{}, fmt.Errorf("open feed: %w", err)
	}
	defer func() { _ = f.Close() }()

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := storage.Open(connectCtx, dbURL)
	if err != nil {
		return ingestion.Summary{}, fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	engine, err := approval.NewEngine(approval.DefaultPolicy(), fx.Default, langdetect.New())
	if err != nil {
		return ingestion.Summary{}, fmt.Errorf("build approval engine: %w", err)
	}
	return ingestion.Run(ctx, f, sourceID, time.Now().UTC(), engine, store, slog.Default())
}
