//go:build integration

package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/shopspring/decimal"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/migrations"
)

var testStore *Store

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18.4", tcpostgres.BasicWaitStrategies())
	if err != nil {
		return fail("start postgres container", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			slog.Error("terminate postgres container", "error", err)
		}
	}()

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fail("container connection string", err)
	}
	if err := migrate(ctx, url); err != nil {
		return fail("migrate test database", err)
	}

	testStore, err = Open(ctx, url)
	if err != nil {
		return fail("open store", err)
	}
	defer testStore.Close()

	return m.Run()
}

func fail(msg string, err error) int {
	slog.Error(msg, "error", err)
	return 1
}

func migrate(ctx context.Context, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func reset(t *testing.T, ctx context.Context) {
	t.Helper()
	const stmt = "TRUNCATE postings, outbox, quarantine RESTART IDENTITY CASCADE"
	if _, err := testStore.pool.Exec(ctx, stmt); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func countRows(t *testing.T, ctx context.Context, table string) int {
	t.Helper()
	var n int
	if err := testStore.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func approvedRecord() PostingRecord {
	salary := decimal.RequireFromString("100000.01")
	return PostingRecord{
		SourceID:        "test-feed",
		IdempotencyKey:  "idem-1",
		Raw:             []byte(`{"title":"Backend Engineer"}`),
		Normalized:      []byte(`{"title":"Backend Engineer"}`),
		Title:           "Backend Engineer",
		Company:         "TechCo",
		City:            "",
		Country:         "",
		Remote:          true,
		SalaryAmount:    &salary,
		SalaryUSDAnnual: &salary,
		SalaryCurrency:  "",
		Status:          "approved",
		Reasons:         []byte(`[]`),
		Market:          "US",
		PolicyVersion:   "v1",
		DecidedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestSaveIsIdempotent(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	r := approvedRecord()

	changed, err := testStore.SaveOutcome(ctx, r)
	if err != nil {
		t.Fatalf("first SaveOutcome() error = %v", err)
	}
	if !changed {
		t.Error("first SaveOutcome() changed = false, want true")
	}

	changed, err = testStore.SaveOutcome(ctx, r)
	if err != nil {
		t.Fatalf("second SaveOutcome() error = %v", err)
	}
	if changed {
		t.Error("second SaveOutcome() changed = true, want false")
	}

	if n := countRows(t, ctx, "postings"); n != 1 {
		t.Errorf("postings count = %d, want 1", n)
	}
	if n := countRows(t, ctx, "outbox"); n != 1 {
		t.Errorf("outbox count = %d, want 1", n)
	}
}

func TestPolicyVersionBumpReevaluates(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	r := approvedRecord()
	if _, err := testStore.SaveOutcome(ctx, r); err != nil {
		t.Fatalf("first SaveOutcome() error = %v", err)
	}

	bumped := r
	bumped.PolicyVersion = "v2"

	changed, err := testStore.SaveOutcome(ctx, bumped)
	if err != nil {
		t.Fatalf("bumped SaveOutcome() error = %v", err)
	}
	if !changed {
		t.Error("bumped SaveOutcome() changed = false, want true")
	}

	var storedPolicy string
	err = testStore.pool.QueryRow(ctx, "SELECT policy_version FROM postings WHERE idempotency_key = $1", r.IdempotencyKey).Scan(&storedPolicy)
	if err != nil {
		t.Fatalf("read policy_version: %v", err)
	}
	if storedPolicy != bumped.PolicyVersion {
		t.Errorf("stored policy_version = %q, want %q", storedPolicy, bumped.PolicyVersion)
	}

	if n := countRows(t, ctx, "outbox"); n != 2 {
		t.Errorf("outbox count = %d, want 2", n)
	}
}

func TestDecimalRoundTrip(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	want := decimal.RequireFromString("100000.01")
	r := approvedRecord()
	r.SalaryAmount = &want
	r.SalaryUSDAnnual = &want

	if _, err := testStore.SaveOutcome(ctx, r); err != nil {
		t.Fatalf("SaveOutcome() error = %v", err)
	}

	var gotAmount, gotAnnual *decimal.Decimal
	err := testStore.pool.QueryRow(ctx, "SELECT salary_amount, salary_usd_annual FROM postings WHERE idempotency_key = $1", r.IdempotencyKey).Scan(&gotAmount, &gotAnnual)
	if err != nil {
		t.Fatalf("read salaries: %v", err)
	}
	if gotAmount == nil || !gotAmount.Equal(want) {
		t.Errorf("salary_amount = %v, want %v", gotAmount, want)
	}
	if gotAnnual == nil || !gotAnnual.Equal(want) {
		t.Errorf("salary_usd_annual = %v, want %v", gotAnnual, want)
	}
}

func TestNullPostedOn(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	r := approvedRecord()
	r.PostedOn = nil

	if _, err := testStore.SaveOutcome(ctx, r); err != nil {
		t.Fatalf("SaveOutcome() error = %v", err)
	}

	var postedOn *civil.Date
	err := testStore.pool.QueryRow(ctx, "SELECT posted_on FROM postings WHERE idempotency_key = $1", r.IdempotencyKey).Scan(&postedOn)
	if err != nil {
		t.Fatalf("read posted_on: %v", err)
	}
	if postedOn != nil {
		t.Errorf("posted_on = %v, want NULL", *postedOn)
	}
}

func TestApprovedInvariantEnforced(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	r := approvedRecord()
	r.Title = ""

	_, err := testStore.SaveOutcome(ctx, r)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("SaveOutcome() error = %v, want ErrInvalid", err)
	}
	if n := countRows(t, ctx, "postings"); n != 0 {
		t.Errorf("postings count = %d, want 0", n)
	}
	if n := countRows(t, ctx, "outbox"); n != 0 {
		t.Errorf("outbox count = %d, want 0", n)
	}
}

func TestQuarantine(t *testing.T) {
	ctx := testContext(t)
	reset(t, ctx)

	element := []byte(`{"title": "Broken", "salary": `)
	q := raw.Quarantined{
		Index: 3,
		Raw:   element,
		Err:   errors.New("unexpected end of JSON input"),
	}

	if err := testStore.Quarantine(ctx, "test-feed", q); err != nil {
		t.Fatalf("Quarantine() error = %v", err)
	}

	var elementIndex int32
	var gotRaw []byte
	var gotErr string
	err := testStore.pool.QueryRow(ctx, "SELECT element_index, raw, error FROM quarantine WHERE source_id = $1", "test-feed").Scan(&elementIndex, &gotRaw, &gotErr)
	if err != nil {
		t.Fatalf("read quarantine row: %v", err)
	}
	if elementIndex != int32(q.Index) {
		t.Errorf("element_index = %d, want %d", elementIndex, q.Index)
	}
	if !bytes.Equal(gotRaw, element) {
		t.Errorf("raw = %q, want %q", gotRaw, element)
	}
	if gotErr != q.Err.Error() {
		t.Errorf("error = %q, want %q", gotErr, q.Err.Error())
	}
}
