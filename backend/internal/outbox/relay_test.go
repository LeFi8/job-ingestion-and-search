package outbox

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

type fakeStore struct {
	rows           []postgres.ClaimedPosting
	deadLettered   []int64
	fnFailed       map[int64]string
	fnErr          error
	limit          int
	blockUntilDone bool
}

func (f *fakeStore) WithClaim(ctx context.Context, limit int, fn func([]postgres.ClaimedPosting) (map[int64]string, error)) ([]int64, error) {
	f.limit = limit
	if f.blockUntilDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	failed, err := fn(f.rows)
	f.fnFailed, f.fnErr = failed, err
	if err != nil {
		return nil, err
	}
	return f.deadLettered, nil
}

func (f *fakeStore) PurgeProcessed(context.Context, time.Duration) (int64, error) { return 0, nil }

type fakeIndexer struct {
	actions []search.Action
	results func([]search.Action) []search.ItemResult
	err     error
}

func (f *fakeIndexer) Bulk(_ context.Context, a []search.Action) ([]search.ItemResult, error) {
	f.actions = a
	if f.err != nil {
		return nil, f.err
	}
	if f.results != nil {
		return f.results(a), nil
	}
	out := make([]search.ItemResult, len(a))
	for i := range a {
		out[i] = search.ItemResult{Version: a[i].Version}
	}
	return out, nil
}

func newRelay(s *fakeStore, ix *fakeIndexer, logs *bytes.Buffer) *Relay {
	return &Relay{Store: s, Index: ix, Batch: 10, Poll: time.Second, Log: slog.New(slog.NewJSONHandler(logs, nil))}
}

func amount(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func approvedRow(id int64, posting string) postgres.ClaimedPosting {
	return postgres.ClaimedPosting{
		OutboxID:         id,
		PostingID:        posting,
		Status:           "approved",
		Title:            "Staff Cloud Architect",
		Company:          "Global Enterprises",
		City:             "London",
		Country:          "GB",
		Remote:           true,
		SalaryUSDAnnual:  107000,
		CurrencyInferred: true,
		PostedOn:         &civil.Date{Year: 2023, Month: 10, Day: 12},
		SalaryAmount:     amount("80000"),
		SalaryCurrency:   "GBP",
		SalaryUnit:       "annual",
	}
}

func TestRelayBuildsActions(t *testing.T) {
	rejected := postgres.ClaimedPosting{OutboxID: 8, PostingID: "p-rejected", Status: "rejected", Title: "Intern"}
	hourly := approvedRow(9, "p-hourly")
	hourly.SalaryAmount, hourly.SalaryUnit, hourly.SalaryCurrency = amount("45.5"), "hourly", "USD"
	noSalary := approvedRow(10, "p-nosalary")
	noSalary.SalaryAmount, noSalary.SalaryUnit, noSalary.SalaryCurrency, noSalary.PostedOn, noSalary.City = nil, "", "", nil, ""
	store := &fakeStore{rows: []postgres.ClaimedPosting{approvedRow(7, "p-approved"), rejected, hourly, noSalary}}
	ix := &fakeIndexer{}

	counts, err := newRelay(store, ix, &bytes.Buffer{}).cycle(context.Background())
	if err != nil || counts.claimed != 4 {
		t.Fatalf("cycle() = %+v, %v; want 4 claimed, nil", counts, err)
	}

	want := []search.Action{
		{Version: 7, JobID: "p-approved", Doc: &search.Doc{
			JobID: "p-approved", Title: "Staff Cloud Architect", Company: "Global Enterprises", City: "London", Country: "GB",
			Remote: true, SalaryUSDAnnual: 107000, SalaryDisplay: "80000.00 GBP / year", CurrencyInferred: true, PostedOn: "2023-10-12",
		}},
		{Version: 8, JobID: "p-rejected"},
		{Version: 9, JobID: "p-hourly", Doc: &search.Doc{
			JobID: "p-hourly", Title: "Staff Cloud Architect", Company: "Global Enterprises", City: "London", Country: "GB",
			Remote: true, SalaryUSDAnnual: 107000, SalaryDisplay: "45.50 USD / hour", CurrencyInferred: true, PostedOn: "2023-10-12",
		}},
		{Version: 10, JobID: "p-nosalary", Doc: &search.Doc{
			JobID: "p-nosalary", Title: "Staff Cloud Architect", Company: "Global Enterprises", Country: "GB",
			Remote: true, SalaryUSDAnnual: 107000, CurrencyInferred: true,
		}},
	}
	if !reflect.DeepEqual(ix.actions, want) {
		t.Errorf("actions = %+v\nwant %+v", ix.actions, want)
	}
	if len(store.fnFailed) != 0 {
		t.Errorf("failed = %v, want none", store.fnFailed)
	}
}

func TestRelayFailsApprovedRowWithUnknownSalaryUnit(t *testing.T) {
	bad := approvedRow(3, "p-bad")
	bad.SalaryUnit = "monthly"
	store := &fakeStore{rows: []postgres.ClaimedPosting{bad, approvedRow(4, "p-ok")}}
	ix := &fakeIndexer{}

	if _, err := newRelay(store, ix, &bytes.Buffer{}).cycle(context.Background()); err != nil {
		t.Fatalf("cycle() error = %v", err)
	}
	if len(ix.actions) != 1 || ix.actions[0].JobID != "p-ok" {
		t.Errorf("actions = %+v, want only p-ok", ix.actions)
	}
	if msg := store.fnFailed[3]; !strings.Contains(msg, "monthly") || len(store.fnFailed) != 1 {
		t.Errorf("failed = %v, want only id 3 mentioning the unit", store.fnFailed)
	}
}

func TestRelayReportsItemFailures(t *testing.T) {
	store := &fakeStore{rows: []postgres.ClaimedPosting{approvedRow(1, "a"), approvedRow(2, "b"), approvedRow(3, "c")}}
	ix := &fakeIndexer{results: func(a []search.Action) []search.ItemResult {
		out := make([]search.ItemResult, len(a))
		out[1].Err = errors.New("bulk item b: status 400: mapper_parsing_exception: bad")
		return out
	}}

	if _, err := newRelay(store, ix, &bytes.Buffer{}).cycle(context.Background()); err != nil {
		t.Fatalf("cycle() error = %v", err)
	}
	want := map[int64]string{2: "bulk item b: status 400: mapper_parsing_exception: bad"}
	if !reflect.DeepEqual(store.fnFailed, want) {
		t.Errorf("failed = %v, want %v", store.fnFailed, want)
	}
}

func TestRelayRequestFailureRollsBack(t *testing.T) {
	store := &fakeStore{rows: []postgres.ClaimedPosting{approvedRow(1, "a")}}
	ix := &fakeIndexer{err: errors.New("connection refused")}
	var logs bytes.Buffer

	counts, err := newRelay(store, ix, &logs).cycle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("cycle() error = %v, want the bulk error", err)
	}
	if store.fnErr == nil || store.fnFailed != nil {
		t.Errorf("fn = (%v, %v), want error and no failed map", store.fnFailed, store.fnErr)
	}
	if counts.claimed != 1 {
		t.Errorf("claimed = %d, want 1", counts.claimed)
	}
	if !strings.Contains(logs.String(), `"msg":"outbox.batch"`) || !strings.Contains(logs.String(), "connection refused") {
		t.Errorf("logs = %s, want outbox.batch with the error", logs.String())
	}
}

func TestRelayLogsDeadLettered(t *testing.T) {
	store := &fakeStore{rows: []postgres.ClaimedPosting{approvedRow(1, "a"), approvedRow(2, "b")}, deadLettered: []int64{2}}
	ix := &fakeIndexer{results: func(a []search.Action) []search.ItemResult {
		out := make([]search.ItemResult, len(a))
		out[1].Err = errors.New("poison")
		return out
	}}
	var logs bytes.Buffer

	if _, err := newRelay(store, ix, &logs).cycle(context.Background()); err != nil {
		t.Fatalf("cycle() error = %v", err)
	}
	var line string
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, `"msg":"outbox.dead_lettered"`) {
			line = l
		}
	}
	if !strings.Contains(line, `"outbox_id":2`) || !strings.Contains(line, "poison") {
		t.Errorf("dead_lettered log = %q, want outbox_id 2 and error text", line)
	}
}

func TestRelayEmptyClaimIsSilent(t *testing.T) {
	var logs bytes.Buffer
	ix := &fakeIndexer{}
	counts, err := newRelay(&fakeStore{}, ix, &logs).cycle(context.Background())
	if counts != (batchCounts{}) || err != nil || logs.Len() != 0 || ix.actions != nil {
		t.Errorf("cycle() = %+v, %v, logs %q, actions %v; want idle no-op", counts, err, logs.String(), ix.actions)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{5, 32 * time.Second},
		{6, time.Minute},
		{200, time.Minute},
	}
	for _, tt := range tests {
		if got := backoff(time.Second, tt.failures); got != tt.want {
			t.Errorf("backoff(1s, %d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestRelayLogsCycleDeadline(t *testing.T) {
	var logs bytes.Buffer
	r := newRelay(&fakeStore{blockUntilDone: true}, &fakeIndexer{}, &logs)
	r.cycleTimeout = 10 * time.Millisecond

	_, err := r.cycle(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cycle() error = %v, want deadline exceeded", err)
	}
	if !strings.Contains(logs.String(), `"msg":"outbox.batch"`) || !strings.Contains(logs.String(), "deadline exceeded") {
		t.Errorf("logs = %q, want outbox.batch with the deadline error", logs.String())
	}
}

func TestRelayStaysQuietOnShutdown(t *testing.T) {
	var logs bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := newRelay(&fakeStore{blockUntilDone: true}, &fakeIndexer{}, &logs)

	if _, err := r.cycle(ctx); err == nil {
		t.Fatal("cycle() error = nil, want cancellation error")
	}
	if logs.Len() != 0 {
		t.Errorf("logs = %q, want none on shutdown", logs.String())
	}
}

func TestNextWait(t *testing.T) {
	const poll = time.Second
	const batch = 10
	tests := []struct {
		name         string
		counts       batchCounts
		err          error
		failures     int
		wantWait     time.Duration
		wantFailures int
	}{
		{"full clean batch goes again immediately", batchCounts{claimed: batch, indexed: batch}, nil, 3, 0, 0},
		{"full batch with a failed item waits poll", batchCounts{claimed: batch, indexed: batch - 1, failed: 1}, nil, 0, poll, 0},
		{"partial batch waits poll", batchCounts{claimed: 3, indexed: 3}, nil, 2, poll, 0},
		{"idle waits poll", batchCounts{}, nil, 0, poll, 0},
		{"error backs off", batchCounts{claimed: batch}, errors.New("down"), 0, 2 * poll, 1},
		{"repeated error backs off further", batchCounts{}, errors.New("down"), 1, 4 * poll, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait, failures := nextWait(tt.counts, batch, poll, tt.failures, tt.err)
			if wait != tt.wantWait || failures != tt.wantFailures {
				t.Errorf("nextWait() = %v, %d; want %v, %d", wait, failures, tt.wantWait, tt.wantFailures)
			}
		})
	}
}
