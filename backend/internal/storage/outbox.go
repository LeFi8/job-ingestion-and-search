package storage

import (
	"context"
	"fmt"
	"slices"
	"time"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage/sqlc"
)

type ClaimedPosting struct {
	OutboxID         int64
	PostingID        string
	Status           string
	Title            string
	Company          string
	City             string
	Country          string
	Remote           bool
	SalaryUSDAnnual  int64
	CurrencyInferred bool
	PostedOn         *civil.Date
	SalaryAmount     *decimal.Decimal
	SalaryCurrency   string
	SalaryUnit       string
}

func (s *Store) WithClaim(ctx context.Context, limit int, fn func([]ClaimedPosting) (map[int64]string, error)) ([]int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlc.New(tx)
	rows, err := q.ClaimOutbox(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	claimed := make([]ClaimedPosting, len(rows))
	for i, r := range rows {
		claimed[i] = claimedPosting(r)
	}

	failed, err := fn(claimed)
	if err != nil {
		return nil, err
	}

	failedIDs := make([]int64, 0, len(failed))
	for id := range failed {
		failedIDs = append(failedIDs, id)
	}
	slices.Sort(failedIDs)
	processed := make([]int64, 0, len(claimed))
	for _, c := range claimed {
		if _, bad := failed[c.OutboxID]; !bad {
			processed = append(processed, c.OutboxID)
		}
	}
	for _, id := range failedIDs {
		if !slices.ContainsFunc(claimed, func(c ClaimedPosting) bool { return c.OutboxID == id }) {
			return nil, fmt.Errorf("failed outbox id %d was not claimed", id)
		}
	}

	if len(processed) > 0 {
		if err := q.MarkProcessed(ctx, processed); err != nil {
			return nil, fmt.Errorf("mark outbox processed: %w", err)
		}
	}
	var deadLettered []int64
	for _, id := range failedIDs {
		msg := failed[id]
		dead, err := q.MarkFailed(ctx, sqlc.MarkFailedParams{ID: id, LastError: &msg})
		if err != nil {
			return nil, fmt.Errorf("mark outbox %d failed: %w", id, err)
		}
		if dead {
			deadLettered = append(deadLettered, id)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return deadLettered, nil
}

func claimedPosting(r sqlc.ClaimOutboxRow) ClaimedPosting {
	return ClaimedPosting{
		OutboxID:         r.OutboxID,
		PostingID:        r.PostingID.String(),
		Status:           r.Status,
		Title:            r.Title,
		Company:          r.Company,
		City:             ptr.ValueOr(r.City, ""),
		Country:          ptr.ValueOr(r.Country, ""),
		Remote:           r.Remote,
		SalaryUSDAnnual:  r.SalaryUsdAnnual,
		CurrencyInferred: r.CurrencyInferred,
		PostedOn:         r.PostedOn,
		SalaryAmount:     r.SalaryAmount,
		SalaryCurrency:   ptr.ValueOr(r.SalaryCurrency, ""),
		SalaryUnit:       ptr.ValueOr(r.SalaryUnit, ""),
	}
}

func (s *Store) PurgeProcessed(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := pgtype.Timestamptz{Time: time.Now().Add(-olderThan), Valid: true}
	n, err := sqlc.New(s.pool).PurgeProcessed(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge processed outbox: %w", err)
	}
	return n, nil
}

func (s *Store) EnqueueAll(ctx context.Context) (int64, error) {
	n, err := sqlc.New(s.pool).EnqueueAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("enqueue all postings: %w", err)
	}
	return n, nil
}
