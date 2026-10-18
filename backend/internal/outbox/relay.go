package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

const (
	cycleTimeout   = 30 * time.Second
	maxBackoff     = time.Minute
	statusApproved = "approved"
)

type Store interface {
	WithClaim(ctx context.Context, limit int, fn func([]postgres.ClaimedPosting) (failed map[int64]string, err error)) (deadLettered []int64, err error)
	PurgeProcessed(ctx context.Context, olderThan time.Duration) (int64, error)
}

type JobIndexer interface {
	Bulk(ctx context.Context, a []search.Action) ([]search.ItemResult, error)
}

type Relay struct {
	Store Store
	Index JobIndexer
	Batch int
	Poll  time.Duration
	Log   *slog.Logger

	cycleTimeout time.Duration
}

type batchCounts struct {
	claimed, indexed, deleted, failed int
}

func (r *Relay) Run(ctx context.Context) error {
	if r.Batch <= 0 || r.Poll <= 0 {
		return fmt.Errorf("relay needs positive batch (%d) and poll (%s)", r.Batch, r.Poll)
	}
	failures := 0
	for {
		counts, err := r.cycle(ctx)
		if ctx.Err() != nil {
			return nil
		}
		var wait time.Duration
		wait, failures = nextWait(counts, r.Batch, r.Poll, failures, err)
		if wait == 0 {
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func nextWait(counts batchCounts, batch int, poll time.Duration, failures int, err error) (time.Duration, int) {
	if err != nil {
		failures++
		return backoff(poll, failures), failures
	}
	if counts.claimed >= batch && counts.failed == 0 {
		return 0, 0
	}
	return poll, 0
}

func backoff(poll time.Duration, failures int) time.Duration {
	wait := poll
	for range failures {
		wait *= 2
		if wait >= maxBackoff {
			return maxBackoff
		}
	}
	return wait
}

func (r *Relay) cycle(ctx context.Context) (batchCounts, error) {
	timeout := r.cycleTimeout
	if timeout == 0 {
		timeout = cycleTimeout
	}
	cycleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var counts batchCounts
	var failedErrors map[int64]string
	deadLettered, err := r.Store.WithClaim(cycleCtx, r.Batch, func(rows []postgres.ClaimedPosting) (map[int64]string, error) {
		counts.claimed = len(rows)
		if len(rows) == 0 {
			return nil, nil
		}
		failed, err := r.relay(cycleCtx, rows, &counts)
		failedErrors = failed
		return failed, err
	})

	if err != nil {
		if ctx.Err() == nil {
			r.logBatch(counts, err)
		}
		return counts, fmt.Errorf("relay cycle: %w", err)
	}
	for _, id := range deadLettered {
		r.Log.Error("outbox.dead_lettered", "outbox_id", id, "error", failedErrors[id])
	}
	if counts.claimed > 0 {
		r.logBatch(counts, nil)
	}
	return counts, nil
}

func (r *Relay) relay(ctx context.Context, rows []postgres.ClaimedPosting, counts *batchCounts) (map[int64]string, error) {
	failed := map[int64]string{}
	actions := make([]search.Action, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		action, err := buildAction(row)
		if err != nil {
			failed[row.OutboxID] = err.Error()
			continue
		}
		actions = append(actions, action)
		ids = append(ids, row.OutboxID)
	}

	if len(actions) > 0 {
		results, err := r.Index.Bulk(ctx, actions)
		if err != nil {
			return nil, fmt.Errorf("bulk: %w", err)
		}
		if len(results) != len(actions) {
			return nil, fmt.Errorf("bulk returned %d results for %d actions", len(results), len(actions))
		}
		for i, res := range results {
			switch {
			case res.Err != nil:
				failed[ids[i]] = res.Err.Error()
			case actions[i].Doc != nil:
				counts.indexed++
			default:
				counts.deleted++
			}
		}
	}
	counts.failed = len(failed)
	return failed, nil
}

func (r *Relay) logBatch(c batchCounts, err error) {
	attrs := []any{"claimed", c.claimed, "indexed", c.indexed, "deleted", c.deleted, "failed", c.failed}
	if err != nil {
		r.Log.Error("outbox.batch", append(attrs, "error", err)...)
		return
	}
	r.Log.Info("outbox.batch", attrs...)
}

func buildAction(row postgres.ClaimedPosting) (search.Action, error) {
	if row.Status != statusApproved {
		return search.Action{Version: row.OutboxID, JobID: row.PostingID}, nil
	}
	display, err := posting.SalaryDisplay(row.SalaryAmount, row.SalaryCurrency, posting.PayUnit(row.SalaryUnit))
	if err != nil {
		return search.Action{}, fmt.Errorf("posting %s: %w", row.PostingID, err)
	}
	doc := &search.Doc{
		JobID:            row.PostingID,
		Title:            row.Title,
		Company:          row.Company,
		City:             row.City,
		Country:          row.Country,
		Remote:           row.Remote,
		SalaryUSDAnnual:  row.SalaryUSDAnnual,
		SalaryDisplay:    display,
		CurrencyInferred: row.CurrencyInferred,
	}
	if row.PostedOn != nil {
		doc.PostedOn = row.PostedOn.String()
	}
	return search.Action{Version: row.OutboxID, JobID: row.PostingID, Doc: doc}, nil
}
