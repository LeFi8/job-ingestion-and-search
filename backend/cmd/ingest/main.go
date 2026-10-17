package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/config"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/langdetect"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
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

	if *reindex {
		reindexAll(cfg.DatabaseURL)
		return
	}
	path := flag.Arg(0)
	sourceID := *source
	if sourceID == "" {
		sourceID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	sum, err := run(context.Background(), cfg.DatabaseURL, path, sourceID)
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

func reindexAll(dbURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := postgres.Open(ctx, dbURL)
	if err != nil {
		slog.Error("reindex failed", "error", fmt.Errorf("open store: %w", err))
		os.Exit(1)
	}
	defer store.Close()

	n, err := store.EnqueueAll(ctx)
	if err != nil {
		slog.Error("reindex failed", "error", err)
		os.Exit(1)
	}
	slog.Info("reindex enqueued", "enqueued", n)
}

func run(ctx context.Context, dbURL, path, sourceID string) (ingestion.Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return ingestion.Summary{}, fmt.Errorf("open feed: %w", err)
	}
	defer func() { _ = f.Close() }()

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := postgres.Open(connectCtx, dbURL)
	if err != nil {
		return ingestion.Summary{}, fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, langdetect.New(langdetect.MinRelativeDistance))
	return ingestion.Run(ctx, f, sourceID, time.Now().UTC(), engine, store, slog.Default())
}
