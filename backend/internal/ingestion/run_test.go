package ingestion_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type fakeSaver struct {
	records     []postgres.PostingRecord
	quarantined []raw.Quarantined
	seen        map[string]bool
	saveErr     func(r postgres.PostingRecord) error
	calls       int
}

func newFakeSaver() *fakeSaver {
	return &fakeSaver{seen: map[string]bool{}}
}

func (f *fakeSaver) SaveOutcome(_ context.Context, r postgres.PostingRecord) (bool, error) {
	f.calls++
	if f.saveErr != nil {
		if err := f.saveErr(r); err != nil {
			return false, err
		}
	}
	f.records = append(f.records, r)
	if f.seen[r.IdempotencyKey] {
		return false, nil
	}
	f.seen[r.IdempotencyKey] = true
	return true, nil
}

func (f *fakeSaver) Quarantine(_ context.Context, _ string, q raw.Quarantined) error {
	f.quarantined = append(f.quarantined, q)
	return nil
}

var testDecidedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func testEngine() *approval.Engine {
	detector := fakeDetector{
		"Rejoignez notre équipe pour concevoir des modèles de machine learning avancés.": {lang: posting.French, reliable: true},
		"Entwickeln Sie mobile Anwendungen für iOS und Android.":                         {lang: posting.German, reliable: true},
	}
	return approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)
}

func fixtureElements(t *testing.T) []json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "pipeline_fixture.json"))
	if err != nil {
		t.Fatalf("read pipeline_fixture.json: %v", err)
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(data, &elems); err != nil {
		t.Fatalf("unmarshal pipeline_fixture.json: %v", err)
	}
	return elems
}

func feed(elems ...string) string {
	return "[" + strings.Join(elems, ",") + "]"
}

func validElement(overrides string) string {
	return fmt.Sprintf(`{"title":"Backend Engineer","description":"Build services in Go.","company":"NextGen Systems","location":{"city":"Austin","state":"TX","country":"USA"},"salary":{"value":145000,"currency":"USD"},"employment_type":"Full-Time","posting_date":"2023-10-03","company_type":"Direct Employer","language":"English","remote":false%s}`, overrides)
}

func runFeed(t *testing.T, input string, s ingestion.Saver, log *slog.Logger) (ingestion.Summary, error) {
	t.Helper()
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	return ingestion.Run(context.Background(), strings.NewReader(input), "test-feed", testDecidedAt, testEngine(), s, log)
}

func TestRunFixture(t *testing.T) {
	expected := loadExpectedDecisions(t)
	var wantApproved, wantRejected int
	for _, e := range expected {
		if e.Approved {
			wantApproved++
		} else {
			wantRejected++
		}
	}
	input := feed(toStrings(fixtureElements(t))...)
	s := newFakeSaver()

	got, err := runFeed(t, input, s, nil)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if want := (ingestion.Summary{Approved: wantApproved, Rejected: wantRejected}); got != want {
		t.Errorf("first Run() = %+v, want %+v", got, want)
	}

	got, err = runFeed(t, input, s, nil)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if want := (ingestion.Summary{Unchanged: len(expected)}); got != want {
		t.Errorf("second Run() = %+v, want %+v", got, want)
	}

	first := s.records[0]
	if first.Status != "approved" || first.SourceID != "test-feed" || !first.DecidedAt.Equal(testDecidedAt) {
		t.Errorf("first record = status %q source %q decidedAt %v", first.Status, first.SourceID, first.DecidedAt)
	}
	if first.PolicyVersion != approval.PolicyVersion || first.Market != "US" {
		t.Errorf("first record policy %q market %q", first.PolicyVersion, first.Market)
	}
	if first.SalaryUSDAnnual == nil || first.SalaryUSDAnnual.String() != "145000" {
		t.Errorf("first record SalaryUSDAnnual = %v, want 145000", first.SalaryUSDAnnual)
	}
	if string(first.Reasons) != "[]" {
		t.Errorf("first record Reasons = %s, want []", first.Reasons)
	}
	if !bytes.Equal(first.Raw, fixtureElements(t)[0]) {
		t.Errorf("first record Raw differs from fixture element")
	}
}

func toStrings(elems []json.RawMessage) []string {
	out := make([]string, len(elems))
	for i, e := range elems {
		out[i] = string(e)
	}
	return out
}

func TestRunTruncated(t *testing.T) {
	elems := fixtureElements(t)
	input := "[" + string(elems[0]) + "," + string(elems[1]) + "," + string(elems[2])[:20]
	s := newFakeSaver()

	got, err := runFeed(t, input, s, nil)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("Run() error = %v, want wrapping io.ErrUnexpectedEOF", err)
	}
	if s.calls != 0 || len(s.quarantined) != 0 {
		t.Errorf("saver calls = %d, quarantined = %d, want 0, 0", s.calls, len(s.quarantined))
	}
	if got != (ingestion.Summary{}) {
		t.Errorf("Run() = %+v, want zero Summary", got)
	}
}

func TestRunMalformed(t *testing.T) {
	input := feed(validElement(""), `"just a string"`, `[1, 2]`, `null`, `{"title": 5}`, validElement(`,"posting_date":"2023-10-04"`))
	s := newFakeSaver()

	got, err := runFeed(t, input, s, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Quarantined != 4 || got.Approved != 2 {
		t.Errorf("Run() = %+v, want Quarantined 4, Approved 2", got)
	}
	if len(s.quarantined) != 4 {
		t.Fatalf("len(quarantined) = %d, want 4", len(s.quarantined))
	}
	for i, q := range s.quarantined {
		if q.Index != i+1 {
			t.Errorf("quarantined[%d].Index = %d, want %d", i, q.Index, i+1)
		}
	}
	if len(s.records) != 2 {
		t.Errorf("len(records) = %d, want 2", len(s.records))
	}
}

func TestRunPoisonRecords(t *testing.T) {
	huge := validElement(`,"title":"Salary Overflow","salary":1e20`)
	nul := `{"title":"Dev\u0000eloper","description":"Build \u0000 services in Go.","company":"NextGen Systems","location":{"city":"Austin","state":"TX","country":"USA"},"salary":{"value":145000,"currency":"USD"},"employment_type":"Full-Time","posting_date":"2023-10-03","company_type":"Direct Employer","language":"English","remote":false}`
	s := newFakeSaver()

	got, err := runFeed(t, feed(validElement(""), huge, nul, validElement(`,"posting_date":"2023-10-04"`)), s, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Approved+got.Rejected != 4 || got.Rejected != 1 {
		t.Errorf("Run() = %+v, want 3 approved, 1 rejected", got)
	}

	hugeRec := s.records[1]
	if hugeRec.Status != "rejected" || !strings.Contains(string(hugeRec.Reasons), "salary_invalid") {
		t.Errorf("1e20 record status %q reasons %s, want rejected with salary_invalid", hugeRec.Status, hugeRec.Reasons)
	}
	if hugeRec.SalaryAmount != nil {
		t.Errorf("1e20 record SalaryAmount = %v, want nil", hugeRec.SalaryAmount)
	}

	nulRec := s.records[2]
	if nulRec.Title != "Developer" {
		t.Errorf("NUL record Title = %q, want Developer", nulRec.Title)
	}
	if !bytes.Equal(nulRec.Raw, []byte(nul)) {
		t.Errorf("NUL record Raw = %q, want input element", nulRec.Raw)
	}
}

func TestRunInvalidRowContinues(t *testing.T) {
	s := newFakeSaver()
	s.saveErr = func(r postgres.PostingRecord) error {
		if r.Title == "Bad Row" {
			return fmt.Errorf("upsert posting: %w", postgres.ErrInvalid)
		}
		return nil
	}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	input := feed(validElement(`,"title":"Bad Row"`), validElement(""))

	got, err := runFeed(t, input, s, log)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := (ingestion.Summary{Approved: 1, Invalid: 1}); got != want {
		t.Errorf("Run() = %+v, want %+v", got, want)
	}
	line := findLogLine(t, &buf, "job.invalid")
	if line["level"] != "ERROR" || line["index"] != float64(0) || line["error"] == nil {
		t.Errorf("job.invalid line = %v", line)
	}
}

func TestRunSaveErrorAborts(t *testing.T) {
	boom := errors.New("connection lost")
	s := newFakeSaver()
	s.saveErr = func(r postgres.PostingRecord) error {
		if r.Title == "Second" {
			return boom
		}
		return nil
	}
	input := feed(validElement(""), validElement(`,"title":"Second"`), validElement(`,"title":"Third"`))

	got, err := runFeed(t, input, s, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("Run() error = %v, want wrapping %v", err, boom)
	}
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("Run() error = %q, want element index 1", err)
	}
	if got.Approved != 1 || s.calls != 2 {
		t.Errorf("Run() = %+v after %d calls, want Approved 1 after 2", got, s.calls)
	}
}

func TestRunLogsRejected(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	input := feed(string(fixtureElements(t)[3]))

	if _, err := runFeed(t, input, newFakeSaver(), log); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	line := findLogLine(t, &buf, "job.rejected")
	reasons, ok := line["reasons"].([]any)
	if !ok || len(reasons) != 3 {
		t.Errorf("job.rejected reasons = %v, want 3 reasons", line["reasons"])
	}
}

func TestRunIdempotencyKey(t *testing.T) {
	s := newFakeSaver()
	input := feed(validElement(""), validElement(""), validElement(`,"location":"Boston, MA, USA"`))

	if _, err := runFeed(t, input, s, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if s.records[0].IdempotencyKey == "" {
		t.Fatal("IdempotencyKey is empty")
	}
	if s.records[0].IdempotencyKey != s.records[1].IdempotencyKey {
		t.Errorf("same input gave different keys %q, %q", s.records[0].IdempotencyKey, s.records[1].IdempotencyKey)
	}
	if s.records[0].IdempotencyKey == s.records[2].IdempotencyKey {
		t.Error("different location gave the same key")
	}
}

func findLogLine(t *testing.T, buf *bytes.Buffer, msg string) map[string]any {
	t.Helper()
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var line map[string]any
		if err := json.Unmarshal([]byte(l), &line); err != nil {
			t.Fatalf("log line %q: %v", l, err)
		}
		if line["msg"] == msg {
			return line
		}
	}
	t.Fatalf("no %q log line in %s", msg, buf.String())
	return nil
}
