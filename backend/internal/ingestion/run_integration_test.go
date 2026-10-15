//go:build integration

package ingestion_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres/pgtest"
)

var testURL string

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	url, stop, err := pgtest.Start(context.Background())
	if err != nil {
		slog.Error("start test database", "error", err)
		return 1
	}
	defer stop()
	testURL = url
	return m.Run()
}

func openDB(t *testing.T) (*postgres.Store, *pgx.Conn, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	store, err := postgres.Open(ctx, testURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(store.Close)

	conn, err := pgx.Connect(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return store, conn, ctx
}

func runInto(t *testing.T, ctx context.Context, store *postgres.Store, input string) ingestion.Summary {
	t.Helper()
	log := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	sum, err := ingestion.Run(ctx, strings.NewReader(input), "test-feed", testDecidedAt, testEngine(), store, log)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return sum
}

func outboxCount(t *testing.T, ctx context.Context, conn *pgx.Conn) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM outbox").Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}

func TestRunAgainstPostgres(t *testing.T) {
	store, conn, ctx := openDB(t)
	input := feed(toStrings(fixtureElements(t))...)
	before := outboxCount(t, ctx, conn)

	first := runInto(t, ctx, store, input)
	second := runInto(t, ctx, store, input)

	total := first.Approved + first.Rejected
	if total != 5 || first.Invalid != 0 {
		t.Fatalf("first Run() = %+v, want 5 stored", first)
	}
	if want := (ingestion.Summary{Unchanged: total}); second != want {
		t.Errorf("second Run() = %+v, want %+v", second, want)
	}
	if got := outboxCount(t, ctx, conn) - before; got != total {
		t.Errorf("outbox rows added = %d, want %d", got, total)
	}
}

func TestRunPoisonAgainstPostgres(t *testing.T) {
	store, conn, ctx := openDB(t)
	huge := validElement(`,"title":"Salary Overflow","salary":1e20`)
	nul := `{"title":"Dev\u0000eloper","description":"Build \u0000 services in Go.","company":"NextGen Systems","location":{"city":"Austin","state":"TX","country":"USA"},"salary":{"value":145000,"currency":"USD"},"employment_type":"Full-Time","posting_date":"2023-10-03","company_type":"Direct Employer","language":"English","remote":false}`

	sum := runInto(t, ctx, store, feed(validElement(""), huge, nul, validElement(`,"posting_date":"2023-10-04"`)))
	if want := (ingestion.Summary{Approved: 3, Rejected: 1}); sum != want {
		t.Errorf("Run() = %+v, want %+v", sum, want)
	}

	var status, reasons string
	err := conn.QueryRow(ctx, "SELECT status, reasons::text FROM postings WHERE title = 'Salary Overflow'").Scan(&status, &reasons)
	if err != nil {
		t.Fatalf("read overflow row: %v", err)
	}
	if status != "rejected" || !strings.Contains(reasons, "salary_invalid") {
		t.Errorf("overflow row status %q reasons %s", status, reasons)
	}

	var gotRaw []byte
	if err := conn.QueryRow(ctx, "SELECT raw FROM postings WHERE title = 'Developer'").Scan(&gotRaw); err != nil {
		t.Fatalf("read NUL row: %v", err)
	}
	if !bytes.Equal(gotRaw, []byte(nul)) {
		t.Errorf("stored raw = %q, want input element", gotRaw)
	}
}

func TestRunDuplicateKeyAgainstPostgres(t *testing.T) {
	store, conn, ctx := openDB(t)
	input := feed(
		validElement(`,"title":"Dup Key"`),
		validElement(`,"title":"Dup Key","salary":{"value":150000,"currency":"USD"}`),
	)
	before := outboxCount(t, ctx, conn)

	first := runInto(t, ctx, store, input)
	second := runInto(t, ctx, store, input)

	if want := (ingestion.Summary{Approved: 1, Duplicate: 1}); first != want {
		t.Errorf("first Run() = %+v, want %+v", first, want)
	}
	if want := (ingestion.Summary{Unchanged: 1, Duplicate: 1}); second != want {
		t.Errorf("second Run() = %+v, want %+v", second, want)
	}
	if got := outboxCount(t, ctx, conn) - before; got != 1 {
		t.Errorf("outbox rows added = %d, want 1", got)
	}
	var salary string
	if err := conn.QueryRow(ctx, "SELECT salary_amount::text FROM postings WHERE title = 'Dup Key'").Scan(&salary); err != nil {
		t.Fatalf("read Dup Key row: %v", err)
	}
	if salary != "145000.00" {
		t.Errorf("salary_amount = %s, want 145000.00 (first element)", salary)
	}
}
