//go:build integration

package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search/estest"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage/pgtest"
)

var (
	pgURL string
	esURL string
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	url, stopPG, err := pgtest.Start(ctx)
	if err != nil {
		slog.Error("start test database", "error", err)
		return 1
	}
	defer stopPG()
	pgURL = url

	url, stopES, err := estest.Start(ctx)
	if err != nil {
		slog.Error("start test elasticsearch", "error", err)
		return 1
	}
	defer stopES()
	esURL = url

	return m.Run()
}

type env struct {
	ctx   context.Context
	store *storage.Store
	conn  *pgx.Conn
	es    *search.Client
	logs  *bytes.Buffer
}

func setup(t *testing.T) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	store, err := storage.Open(ctx, pgURL)
	if err != nil {
		t.Fatalf("postgres.Open() error = %v", err)
	}
	t.Cleanup(store.Close)

	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	if _, err := conn.Exec(ctx, "DELETE FROM postings"); err != nil {
		t.Fatalf("reset postings: %v", err)
	}

	es, err := search.New(esURL)
	if err != nil {
		t.Fatalf("search.New() error = %v", err)
	}
	httpDo(t, ctx, http.MethodDelete, "/jobs_v1?ignore_unavailable=true")
	if err := es.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex() error = %v", err)
	}
	return &env{ctx: ctx, store: store, conn: conn, es: es, logs: &bytes.Buffer{}}
}

func httpDo(t *testing.T, ctx context.Context, method, path string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, esURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d: %s", method, path, resp.StatusCode, body)
	}
	return body
}

func (e *env) relay(ix JobIndexer, store Store) *Relay {
	if ix == nil {
		ix = e.es
	}
	if store == nil {
		store = e.store
	}
	return &Relay{Store: store, Index: ix, Batch: 100, Poll: time.Second, Log: slog.New(slog.NewJSONHandler(e.logs, nil))}
}

func (e *env) docCount(t *testing.T) int {
	t.Helper()
	httpDo(t, e.ctx, http.MethodPost, "/jobs/_refresh")
	var out struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(httpDo(t, e.ctx, http.MethodGet, "/jobs/_count"), &out); err != nil {
		t.Fatalf("decode count: %v", err)
	}
	return out.Count
}

func (e *env) outboxCount(t *testing.T, where string) int {
	t.Helper()
	var n int
	if err := e.conn.QueryRow(e.ctx, "SELECT count(*) FROM outbox WHERE "+where).Scan(&n); err != nil {
		t.Fatalf("count outbox where %s: %v", where, err)
	}
	return n
}

type fakeDetector map[string]posting.Language

func (f fakeDetector) Detect(text string) (posting.Language, bool) {
	if lang, ok := f[text]; ok {
		return lang, true
	}
	return posting.English, true
}

func (e *env) ingest(t *testing.T, sourceID, input string) {
	t.Helper()
	detector := fakeDetector{
		"Rejoignez notre équipe pour concevoir des modèles de machine learning avancés.": posting.French,
		"Entwickeln Sie mobile Anwendungen für iOS und Android.":                         posting.German,
	}
	engine, err := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ingestion.Run(e.ctx, strings.NewReader(input), sourceID, time.Now().UTC(), engine, e.store, log); err != nil {
		t.Fatalf("ingestion.Run() error = %v", err)
	}
}

func fixtureFeed(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "ingestion", "testdata", "pipeline_fixture.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func element(employmentType string) string {
	return fmt.Sprintf(`[{"title":"Backend Engineer","description":"Build services in Go.","company":"NextGen Systems","location":{"city":"Austin","state":"TX","country":"USA"},"salary":{"value":145000,"currency":"USD"},"employment_type":%q,"posting_date":"2023-10-03","company_type":"Direct Employer","language":"English","remote":false}]`, employmentType)
}

func mustCycle(t *testing.T, ctx context.Context, r *Relay) int {
	t.Helper()
	counts, err := r.cycle(ctx)
	if err != nil {
		t.Fatalf("cycle() error = %v", err)
	}
	return counts.claimed
}

func TestRelayIndexesApprovedDeletesRejected(t *testing.T) {
	e := setup(t)
	e.ingest(t, t.Name(), fixtureFeed(t))

	if claimed := mustCycle(t, e.ctx, e.relay(nil, nil)); claimed != 5 {
		t.Errorf("claimed = %d, want 5", claimed)
	}
	if got := e.docCount(t); got != 3 {
		t.Errorf("ES count = %d, want 3 approved", got)
	}
	if got := e.outboxCount(t, "processed_at IS NOT NULL"); got != 5 {
		t.Errorf("processed outbox rows = %d, want 5", got)
	}
}

func TestApprovedThenRejected(t *testing.T) {
	e := setup(t)
	r := e.relay(nil, nil)

	e.ingest(t, t.Name(), element("Full-Time"))
	mustCycle(t, e.ctx, r)
	if got := e.docCount(t); got != 1 {
		t.Fatalf("ES count after approval = %d, want 1", got)
	}

	e.ingest(t, t.Name(), element("Internship"))
	mustCycle(t, e.ctx, r)
	if got := e.docCount(t); got != 0 {
		t.Errorf("ES count after rejection = %d, want 0", got)
	}
}

type crashAfterFn struct{ Store }

func (c crashAfterFn) WithClaim(ctx context.Context, limit int, fn func([]storage.ClaimedPosting) (map[int64]string, error)) ([]int64, error) {
	return c.Store.WithClaim(ctx, limit, func(rows []storage.ClaimedPosting) (map[int64]string, error) {
		if _, err := fn(rows); err != nil {
			return nil, err
		}
		return nil, errors.New("simulated crash before commit")
	})
}

func TestCrashBeforeCommit(t *testing.T) {
	e := setup(t)
	e.ingest(t, t.Name(), fixtureFeed(t))

	if _, err := e.relay(nil, crashAfterFn{e.store}).cycle(e.ctx); err == nil {
		t.Fatal("cycle() error = nil, want the simulated crash")
	}
	if got := e.outboxCount(t, "processed_at IS NULL AND failed_at IS NULL AND attempts = 0"); got != 5 {
		t.Errorf("pending untouched outbox rows = %d, want 5", got)
	}

	if claimed := mustCycle(t, e.ctx, e.relay(nil, nil)); claimed != 5 {
		t.Errorf("claimed after crash = %d, want 5", claimed)
	}
	if got := e.docCount(t); got != 3 {
		t.Errorf("ES count = %d, want 3", got)
	}
	if got := e.outboxCount(t, "processed_at IS NOT NULL"); got != 5 {
		t.Errorf("processed outbox rows = %d, want 5", got)
	}
}

type poisonIndexer struct{ poisonID string }

func (p poisonIndexer) Bulk(_ context.Context, a []search.Action) ([]search.ItemResult, error) {
	out := make([]search.ItemResult, len(a))
	for i := range a {
		out[i].Version = a[i].Version
		if a[i].JobID == p.poisonID {
			out[i].Err = errors.New("bulk item: status 400: document_parsing_exception: poison")
		}
	}
	return out, nil
}

func TestPoisonDocDeadLetters(t *testing.T) {
	e := setup(t)
	e.ingest(t, t.Name(), fixtureFeed(t))
	var poisonID string
	if err := e.conn.QueryRow(e.ctx, "SELECT posting_id::text FROM outbox ORDER BY id LIMIT 1").Scan(&poisonID); err != nil {
		t.Fatalf("read poison posting: %v", err)
	}
	r := e.relay(poisonIndexer{poisonID}, nil)

	for range 5 {
		mustCycle(t, e.ctx, r)
	}

	if got := e.outboxCount(t, "failed_at IS NOT NULL AND attempts = 5 AND posting_id = '"+poisonID+"'"); got != 1 {
		t.Errorf("dead-lettered poison rows = %d, want 1", got)
	}
	if got := e.outboxCount(t, "processed_at IS NOT NULL"); got != 4 {
		t.Errorf("processed rows = %d, want 4", got)
	}
	if got := strings.Count(e.logs.String(), `"msg":"outbox.dead_lettered"`); got != 1 {
		t.Errorf("outbox.dead_lettered logged %d times, want 1", got)
	}
	if claimed := mustCycle(t, e.ctx, r); claimed != 0 {
		t.Errorf("claimed after dead-letter = %d, want 0", claimed)
	}
}

func TestPurgeProcessed(t *testing.T) {
	e := setup(t)
	e.ingest(t, t.Name(), fixtureFeed(t))
	if _, err := e.conn.Exec(e.ctx, "UPDATE outbox SET created_at = now() - interval '10 days'"); err != nil {
		t.Fatalf("age created_at: %v", err)
	}
	_, err := e.conn.Exec(e.ctx, `
		UPDATE outbox SET processed_at = CASE
			WHEN id = (SELECT min(id) FROM outbox) THEN now() - interval '8 days'
			ELSE now() - interval '1 day'
		END
		WHERE id IN (SELECT id FROM outbox ORDER BY id LIMIT 2)`)
	if err != nil {
		t.Fatalf("age outbox rows: %v", err)
	}

	n, err := e.store.PurgeProcessed(e.ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("PurgeProcessed() error = %v", err)
	}
	if n != 1 {
		t.Errorf("purged = %d, want 1", n)
	}
	if got := e.outboxCount(t, "true"); got != 4 {
		t.Errorf("remaining outbox rows = %d, want 4 (recent processed + 3 pending)", got)
	}
	if got := e.outboxCount(t, "processed_at IS NULL"); got != 3 {
		t.Errorf("pending rows = %d, want 3", got)
	}
}
