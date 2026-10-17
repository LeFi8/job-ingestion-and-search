package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

type fakeSearcher struct {
	called bool
	got    search.Query
	page   search.Page
	err    error
}

func (f *fakeSearcher) Search(_ context.Context, q search.Query) (search.Page, error) {
	f.called = true
	f.got = q
	return f.page, f.err
}

func serve(t *testing.T, s JobSearcher, target string) *httptest.ResponseRecorder {
	t.Helper()
	router, err := NewRouter(s, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
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
