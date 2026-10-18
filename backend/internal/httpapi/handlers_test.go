package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

const testSearchTimeout = 50 * time.Millisecond

type fakeSearcher struct {
	called bool
	block  bool
	got    search.Query
	page   search.Page
	err    error
}

func (f *fakeSearcher) Search(ctx context.Context, q search.Query) (search.Page, error) {
	f.called = true
	f.got = q
	if f.block {
		<-ctx.Done()
		return search.Page{}, ctx.Err()
	}
	return f.page, f.err
}

type fakeJobReader struct {
	called bool
	gotID  uuid.UUID
	job    postgres.Job
	err    error
}

func (f *fakeJobReader) ApprovedJob(_ context.Context, id uuid.UUID) (postgres.Job, error) {
	f.called = true
	f.gotID = id
	return f.job, f.err
}

func newTestRouter(t *testing.T, s JobSearcher, log *slog.Logger) http.Handler {
	t.Helper()
	return newTestRouterWithJobs(t, s, &fakeJobReader{}, log)
}

func newTestRouterWithJobs(t *testing.T, s JobSearcher, jobs JobReader, log *slog.Logger) http.Handler {
	t.Helper()
	router, err := NewRouter(s, jobs, testSearchTimeout, log)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return router
}

func serve(t *testing.T, s JobSearcher, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newTestRouter(t, s, slog.New(slog.DiscardHandler)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func serveJob(t *testing.T, jobs JobReader, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newTestRouterWithJobs(t, &fakeSearcher{}, jobs, slog.New(slog.DiscardHandler)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) Problem {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, wantStatus, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v; body %s", err, rec.Body)
	}
	if p.Status != wantStatus || p.Title == "" {
		t.Errorf("problem = %+v, want status %d and a title", p, wantStatus)
	}
	return p
}

func TestGetJobsRejectsInvalidParams(t *testing.T) {
	tests := []struct {
		query string
		param string
	}{
		{"sort=bogus", "sort"},
		{"per_page=51", "per_page"},
		{"per_page=0", "per_page"},
		{"page=201", "page"},
		{"page=abc", "page"},
		{"country=us", "country"},
		{"country=XX1", "country"},
		{"country=" + strings.Repeat("US&country=", 20) + "CA", "country"},
		{"q=" + strings.Repeat("a", 201), "q"},
	}
	for _, tt := range tests {
		t.Run(tt.query[:min(len(tt.query), 20)], func(t *testing.T) {
			fake := &fakeSearcher{}
			p := decodeProblem(t, serve(t, fake, "/api/jobs?"+tt.query), http.StatusBadRequest)
			if p.Detail == nil || !strings.Contains(*p.Detail, `"`+tt.param+`"`) || strings.HasSuffix(*p.Detail, ": ") {
				t.Errorf("detail = %q, want it to name %q with a reason", ptr.ValueOr(p.Detail, ""), tt.param)
			}
			if fake.called {
				t.Error("searcher was called for an invalid request")
			}
		})
	}
}

func TestGetJobRejectsMalformedID(t *testing.T) {
	fake := &fakeJobReader{}
	decodeProblem(t, serveJob(t, fake, "/api/jobs/not-a-uuid"), http.StatusBadRequest)
	if fake.called {
		t.Error("reader was called for a malformed id")
	}
}

func TestGetJob(t *testing.T) {
	id := uuid.MustParse("0f8fad5b-d9cb-469f-a165-70867728950e")
	amount := decimal.RequireFromString("62.5")
	postedOn := civil.Date{Year: 2026, Month: 3, Day: 14}
	full := postgres.Job{
		PostingID: id.String(), Title: "Backend Engineer", Company: "Acme",
		City: "Austin", Region: "TX", Country: "US", Remote: true,
		SalaryUSDAnnual: 130000, CurrencyInferred: true, PostedOn: &postedOn,
		SalaryAmount: &amount, SalaryCurrency: "CAD", SalaryUnit: "hourly",
		Description: "Build services in Go.", CompanyType: "direct_employer",
	}
	minimal := postgres.Job{
		PostingID: id.String(), Title: "Backend Engineer", Company: "Acme",
		Description: "Build services in Go.", CompanyType: "unknown",
	}

	tests := []struct {
		name   string
		job    postgres.Job
		err    error
		status int
		want   string
	}{
		{"found", full, nil, http.StatusOK, `{
			"id": "0f8fad5b-d9cb-469f-a165-70867728950e", "title": "Backend Engineer", "company": "Acme",
			"city": "Austin", "region": "TX", "country": "US", "remote": true,
			"salary_display": "62.50 CAD / hour", "salary_usd_annual": 130000, "currency_inferred": true,
			"posted_on": "2026-03-14", "description": "Build services in Go.", "company_type": "direct_employer"
		}`},
		{"sparse", minimal, nil, http.StatusOK, `{
			"id": "0f8fad5b-d9cb-469f-a165-70867728950e", "title": "Backend Engineer", "company": "Acme",
			"remote": false, "salary_display": "", "salary_usd_annual": 0, "currency_inferred": false,
			"description": "Build services in Go.", "company_type": "unknown"
		}`},
		{"invalid company type", postgres.Job{PostingID: id.String(), CompanyType: ""}, nil, http.StatusInternalServerError, ""},
		{"not found", postgres.Job{}, postgres.ErrNotFound, http.StatusNotFound, ""},
		{"other error", postgres.Job{}, errors.New("dial tcp pg-internal:5432: connection refused"), http.StatusServiceUnavailable, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeJobReader{job: tt.job, err: tt.err}
			rec := serveJob(t, fake, "/api/jobs/"+id.String())
			if fake.gotID != id {
				t.Errorf("reader got id %s, want %s", fake.gotID, id)
			}
			if tt.status == http.StatusOK {
				assertJSON(t, rec, tt.want)
				return
			}
			decodeProblem(t, rec, tt.status)
			if strings.Contains(rec.Body.String(), "pg-internal") {
				t.Errorf("body leaks the reader error: %s", rec.Body)
			}
		})
	}
}

func TestGetJobsMapsQueryAndHits(t *testing.T) {
	fake := &fakeSearcher{page: search.Page{
		Total: 2,
		Items: []search.Hit{{
			Doc: search.Doc{
				JobID: "job-1", Title: "Backend Engineer", Company: "Acme", Country: "US", Remote: true,
				SalaryUSDAnnual: 150000, SalaryDisplay: "150000.00 USD / year",
			},
			Highlight: []search.Segment{{Text: "Backend "}, {Text: "Engineer", Match: true}},
		}},
		Facets: []search.Facet{{Value: "US", Count: 1}, {Value: "CA", Count: 1}},
	}}

	rec := serve(t, fake, "/api/jobs?q=eng&country=US&country=CA")

	wantQuery := search.Query{Q: "eng", Countries: []string{"US", "CA"}, Page: 1, PerPage: 20}
	if !reflect.DeepEqual(fake.got, wantQuery) {
		t.Errorf("searcher got %+v, want %+v", fake.got, wantQuery)
	}
	assertJSON(t, rec, `{
		"items": [{
			"id": "job-1", "title": "Backend Engineer",
			"title_highlight": [{"text": "Backend ", "match": false}, {"text": "Engineer", "match": true}],
			"company": "Acme", "country": "US", "remote": true,
			"salary_display": "150000.00 USD / year", "salary_usd_annual": 150000, "currency_inferred": false
		}],
		"total": 2, "page": 1, "per_page": 20,
		"facets": {"country": [{"value": "US", "count": 1}, {"value": "CA", "count": 1}]}
	}`)
}

func TestGetJobsEmptyPageHasEmptyArrays(t *testing.T) {
	rec := serve(t, &fakeSearcher{}, "/api/jobs?page=3&per_page=5")
	assertJSON(t, rec, `{"items": [], "total": 0, "page": 3, "per_page": 5, "facets": {"country": []}}`)
}

func TestGetJobsSearchErrorIs503WithoutLeak(t *testing.T) {
	fake := &fakeSearcher{err: errors.New("dial tcp es-internal:9200: connection refused")}
	rec := serve(t, fake, "/api/jobs")
	decodeProblem(t, rec, http.StatusServiceUnavailable)
	if strings.Contains(rec.Body.String(), "es-internal") {
		t.Errorf("body leaks the search error: %s", rec.Body)
	}
}

func TestGetJobsSearchTimeoutIs503(t *testing.T) {
	router := newTestRouter(t, &fakeSearcher{block: true}, slog.New(slog.DiscardHandler))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
		done <- rec
	}()

	select {
	case rec := <-done:
		decodeProblem(t, rec, http.StatusServiceUnavailable)
	case <-time.After(2 * time.Second):
		t.Fatal("no response 2s after the request; the search call has no deadline")
	}
}

func TestGetJobsClientCancelIsNotLoggedAsError(t *testing.T) {
	var logs bytes.Buffer
	router := newTestRouter(t, &fakeSearcher{block: true}, slog.New(slog.NewTextHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/jobs", nil))

	if rec.Code != 499 {
		t.Errorf("status = %d, want 499 (client closed request)", rec.Code)
	}
	if strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "status=499") {
		t.Errorf("access log = %q, want a non-error line with status=499", logs.String())
	}
}

func TestUnknownRoutesAndMethodsUseProblem(t *testing.T) {
	tests := []struct {
		method, target string
		status         int
		allow          string
	}{
		{http.MethodGet, "/api/job", http.StatusNotFound, ""},
		{http.MethodPost, "/api/jobs", http.StatusMethodNotAllowed, "GET"},
	}
	router := newTestRouter(t, &fakeSearcher{}, slog.New(slog.DiscardHandler))
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))
			decodeProblem(t, rec, tt.status)
			if got := rec.Header().Get("Allow"); got != tt.allow {
				t.Errorf("Allow = %q, want %q", got, tt.allow)
			}
		})
	}
}

func assertJSON(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var got, wantValue any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	if !reflect.DeepEqual(got, wantValue) {
		t.Errorf("body = %s\nwant %s", rec.Body, want)
	}
}
