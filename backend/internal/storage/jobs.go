package storage

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage/sqlc"
)

type Job struct {
	PostingID        string
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
	Description      string
	Region           string
	CompanyType      string
}

func (s *Store) ApprovedJob(ctx context.Context, id uuid.UUID) (Job, error) {
	r, err := sqlc.New(s.pool).GetApprovedJob(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get approved job: %w", err)
	}
	return job(r), nil
}

func job(r sqlc.GetApprovedJobRow) Job {
	return Job{
		PostingID:        r.PostingID.String(),
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
		Description:      r.Description,
		Region:           r.Region,
		CompanyType:      r.CompanyType,
	}
}
