package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage/sqlc"
)

type PostingRecord struct {
	SourceID         string
	IdempotencyKey   string
	Raw              []byte
	Normalized       []byte
	Title            string
	Company          string
	City             string
	Country          string
	Remote           bool
	SalaryAmount     *decimal.Decimal
	SalaryCurrency   string
	SalaryUnit       string
	SalaryUSDAnnual  *decimal.Decimal
	CurrencyInferred bool
	FxRate           *decimal.Decimal
	PostedOn         *civil.Date
	Status           string
	Reasons          []byte
	Market           string
	PolicyVersion    string
	DecidedAt        time.Time
}

func (s *Store) SaveOutcome(ctx context.Context, r PostingRecord) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlc.New(tx)
	id, err := q.UpsertPosting(ctx, upsertParams(r))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("upsert posting: %w", mapErr(err))
	}
	if err := q.InsertOutbox(ctx, id); err != nil {
		return false, fmt.Errorf("insert outbox: %w", mapErr(err))
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}
	return true, nil
}

func upsertParams(r PostingRecord) sqlc.UpsertPostingParams {
	return sqlc.UpsertPostingParams{
		SourceID:         r.SourceID,
		IdempotencyKey:   r.IdempotencyKey,
		ContentHash:      contentHash(r),
		Raw:              r.Raw,
		Normalized:       r.Normalized,
		Title:            r.Title,
		Company:          r.Company,
		City:             ptr.NilIfZero(r.City),
		Country:          ptr.NilIfZero(r.Country),
		Remote:           r.Remote,
		SalaryAmount:     r.SalaryAmount,
		SalaryCurrency:   ptr.NilIfZero(r.SalaryCurrency),
		SalaryUnit:       ptr.NilIfZero(r.SalaryUnit),
		SalaryUsdAnnual:  r.SalaryUSDAnnual,
		CurrencyInferred: r.CurrencyInferred,
		FxRate:           r.FxRate,
		PostedOn:         r.PostedOn,
		Status:           r.Status,
		Reasons:          r.Reasons,
		Market:           r.Market,
		PolicyVersion:    r.PolicyVersion,
		DecidedAt:        pgtype.Timestamptz{Time: r.DecidedAt, Valid: true},
	}
}

func contentHash(r PostingRecord) string {
	h := sha256.New()
	h.Write(r.Raw)
	h.Write([]byte(r.PolicyVersion))
	return hex.EncodeToString(h.Sum(nil))
}
