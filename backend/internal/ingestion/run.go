package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage"
)

type Saver interface {
	SaveOutcome(ctx context.Context, r storage.PostingRecord) (changed bool, err error)
	Quarantine(ctx context.Context, sourceID string, q raw.Quarantined) (inserted bool, err error)
}

type Summary struct{ Approved, Rejected, Unchanged, Quarantined, Invalid, Duplicate int }

func Run(ctx context.Context, r io.Reader, sourceID string, decidedAt time.Time, e *approval.Engine, s Saver, log *slog.Logger) (Summary, error) {
	outcomes, quarantined, err := Evaluate(r, sourceID, e)
	if err != nil {
		return Summary{}, err
	}

	var sum Summary
	firstQuarantined := make(map[string]int, len(quarantined))
	for _, q := range quarantined {
		if first, dup := firstQuarantined[string(q.Raw)]; dup {
			sum.Duplicate++
			log.Warn("feed.duplicate_element", "index", q.Index, "first_index", first)
			continue
		}
		firstQuarantined[string(q.Raw)] = q.Index

		inserted, err := s.Quarantine(ctx, sourceID, q)
		if err != nil {
			return sum, fmt.Errorf("quarantine element %d: %w", q.Index, err)
		}
		if !inserted {
			sum.Unchanged++
			continue
		}
		sum.Quarantined++
		log.Warn("feed.quarantined", "index", q.Index, "error", q.Err.Error())
	}

	firstIndex := make(map[string]int, len(outcomes))
	for _, o := range outcomes {
		for _, issue := range o.Issues {
			if issue.Code == posting.DateInvalid {
				log.Warn("job.date_invalid", "index", o.Index, "value", issue.Detail)
			}
		}
		rec, err := newRecord(sourceID, decidedAt, o)
		if err != nil {
			return sum, fmt.Errorf("build record for element %d: %w", o.Index, err)
		}
		if first, dup := firstIndex[rec.IdempotencyKey]; dup {
			sum.Duplicate++
			log.Warn("job.duplicate_key", "index", o.Index, "first_index", first, "idempotency_key", rec.IdempotencyKey)
			continue
		}
		firstIndex[rec.IdempotencyKey] = o.Index

		changed, err := s.SaveOutcome(ctx, rec)
		if errors.Is(err, storage.ErrInvalid) {
			sum.Invalid++
			log.Error("job.invalid", "index", o.Index, "error", err.Error())
			continue
		}
		if err != nil {
			return sum, fmt.Errorf("save element %d: %w", o.Index, err)
		}

		switch {
		case !changed:
			sum.Unchanged++
		case o.Decision.Approved():
			sum.Approved++
		default:
			sum.Rejected++
			log.Info("job.rejected", "index", o.Index, "title", rec.Title, "reasons", reasonCodes(o.Decision.Reasons))
		}
	}

	return sum, nil
}

func newRecord(sourceID string, decidedAt time.Time, o Outcome) (storage.PostingRecord, error) {
	key, err := idempotencyKey(sourceID, o.Raw, o.Posting)
	if err != nil {
		return storage.PostingRecord{}, err
	}
	normalized, err := json.Marshal(o.Posting)
	if err != nil {
		return storage.PostingRecord{}, fmt.Errorf("marshal normalized posting: %w", err)
	}
	reasons := o.Decision.Reasons
	if reasons == nil {
		reasons = []approval.Reason{}
	}
	reasonsJSON, err := json.Marshal(reasons)
	if err != nil {
		return storage.PostingRecord{}, fmt.Errorf("marshal reasons: %w", err)
	}

	rec := storage.PostingRecord{
		SourceID:       sourceID,
		IdempotencyKey: key,
		Raw:            o.RawBytes,
		Normalized:     normalized,
		Title:          o.Posting.Title,
		Company:        o.Posting.Company,
		Remote:         o.Posting.Remote,
		PostedOn:       o.Posting.PostedOn,
		Status:         "rejected",
		Reasons:        reasonsJSON,
		Market:         o.Decision.Market,
		PolicyVersion:  o.Decision.PolicyVersion,
		DecidedAt:      decidedAt,
	}
	if o.Decision.Approved() {
		rec.Status = "approved"
	}
	if loc := o.Posting.Location; loc != nil {
		rec.City = loc.City
		rec.Country = string(loc.Country)
	}
	if sal := o.Posting.Salary; sal != nil {
		amount := sal.Amount
		rec.SalaryAmount = &amount
		rec.SalaryCurrency = string(sal.Currency)
		rec.SalaryUnit = string(sal.Unit)
	}
	if fx := o.Decision.FX; fx != nil {
		rate := fx.Rate
		rec.FxRate = &rate
		rec.CurrencyInferred = fx.Inferred
	}
	rec.SalaryUSDAnnual = salaryUSD(o.Decision)
	return rec, nil
}

func salaryUSD(d approval.Decision) *decimal.Decimal {
	if d.FX == nil {
		return nil
	}
	annual := d.FX.AnnualUSD
	return &annual
}

func reasonCodes(reasons []approval.Reason) []string {
	codes := make([]string, len(reasons))
	for i, r := range reasons {
		codes[i] = string(r.Code)
	}
	return codes
}
