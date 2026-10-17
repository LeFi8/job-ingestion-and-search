//go:build integration

package search

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/search/estest"
)

var testClient *Client

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	url, stop, err := estest.Start(ctx)
	if err != nil {
		return fail("start test elasticsearch", err)
	}
	defer stop()

	testClient, err = New(url)
	if err != nil {
		return fail("new client", err)
	}
	return m.Run()
}

func fail(msg string, err error) int {
	slog.Error(msg, "error", err)
	return 1
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func resetIndex(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := testClient.es.Indices.Delete(indexName).IgnoreUnavailable(true).Do(ctx); err != nil {
		t.Fatalf("delete index: %v", err)
	}
	if err := testClient.EnsureIndex(ctx); err != nil {
		t.Fatalf("EnsureIndex() error = %v", err)
	}
}

func refresh(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := testClient.es.Indices.Refresh().Index(aliasName).Do(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

func seed(t *testing.T, ctx context.Context, docs ...Doc) {
	t.Helper()
	actions := make([]Action, len(docs))
	for i := range docs {
		actions[i] = Action{Version: 1, JobID: docs[i].JobID, Doc: &docs[i]}
	}
	results, err := testClient.Bulk(ctx, actions)
	if err != nil {
		t.Fatalf("seed Bulk() error = %v", err)
	}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("seed item %d error = %v", i, r.Err)
		}
	}
	refresh(t, ctx)
}

func titled(id, title string) Doc {
	return Doc{JobID: id, Title: title, Company: "Co", Country: "US", SalaryUSDAnnual: 100000, SalaryDisplay: "$100,000"}
}

func jobIDs(p Page) []string {
	ids := make([]string, len(p.Items))
	for i, h := range p.Items {
		ids[i] = h.JobID
	}
	return ids
}

func TestSayTSubfieldMapping(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)

	if err := testClient.EnsureIndex(ctx); err != nil {
		t.Errorf("second EnsureIndex() error = %v, want nil for existing index", err)
	}
}

func TestTitleMatching(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	seed(t, ctx,
		titled("be", "Backend Engineer"),
		titled("cpp", "C++ Developer"),
		titled("c", "C Developer"),
		titled("asp", "ASP.NET Developer"),
		titled("fr", "Ingénieur logiciel"),
	)

	tests := []struct {
		q    string
		want []string
	}{
		{"eng", []string{"be"}},
		{"enginer", []string{"be"}},
		{"xyz", []string{}},
		{"c++", []string{"cpp"}},
		{".net", []string{"asp"}},
		{"inge", []string{"fr"}},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			page, err := testClient.Search(ctx, Query{Q: tt.q})
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.q, err)
			}
			if got := jobIDs(page); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search(%q) ids = %v, want %v", tt.q, got, tt.want)
			}
		})
	}
}

func TestHighlightIsPlainText(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	seed(t, ctx,
		titled("img", "Engineer <img src=x onerror=alert(1)>"),
		titled("amp", "R&D Engineer"),
	)

	page, err := testClient.Search(ctx, Query{Q: "engineer"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	got := map[string][]Segment{}
	for _, h := range page.Items {
		got[h.JobID] = h.Highlight
	}
	want := map[string][]Segment{
		"img": {{Text: "Engineer", Match: true}, {Text: " <img src=x onerror=alert(1)>"}},
		"amp": {{Text: "R&D "}, {Text: "Engineer", Match: true}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("highlights = %#v, want %#v", got, want)
	}
}

func TestHighlightMatchesWhatMatched(t *testing.T) {
	ctx := testContext(t)

	tests := []struct {
		q, title string
		want     []string
	}{
		{"eng", "Backend Engineer", []string{"Engineer"}},
		{"enginer", "Backend Engineer", []string{"Engineer"}},
		{"machine lea", "Machine Learning Engineer", []string{"Machine", "Learning"}},
		{"c++", "C++ Developer", []string{"C++"}},
	}
	for _, tt := range tests {
		t.Run(tt.q, func(t *testing.T) {
			resetIndex(t, ctx)
			seed(t, ctx, titled("a", tt.title))

			page, err := testClient.Search(ctx, Query{Q: tt.q})
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.q, err)
			}
			if len(page.Items) != 1 {
				t.Fatalf("Search(%q) returned %d items, want 1", tt.q, len(page.Items))
			}
			var got []string
			for _, s := range page.Items[0].Highlight {
				if s.Match {
					got = append(got, s.Text)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search(%q) matched segments = %v, want %v", tt.q, got, tt.want)
			}
		})
	}
}

func TestSortNullsLast(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	withDate := func(id, date string) Doc {
		d := titled(id, "Engineer "+id)
		d.PostedOn = date
		return d
	}
	seed(t, ctx, withDate("none", ""), withDate("old", "2026-01-01"), withDate("new", "2026-03-01"))

	tests := []struct {
		sort Sort
		want []string
	}{
		{SortDateDesc, []string{"new", "old", "none"}},
		{SortDateAsc, []string{"old", "new", "none"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.sort), func(t *testing.T) {
			page, err := testClient.Search(ctx, Query{Sort: tt.sort})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if got := jobIDs(page); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search(sort=%s) ids = %v, want %v", tt.sort, got, tt.want)
			}
		})
	}
}

func TestSortSalary(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	withSalary := func(id string, salary int64) Doc {
		d := titled(id, "Engineer "+id)
		d.SalaryUSDAnnual = salary
		return d
	}
	seed(t, ctx, withSalary("mid", 100), withSalary("high", 300), withSalary("low", 50))

	tests := []struct {
		sort Sort
		want []string
	}{
		{SortSalaryDesc, []string{"high", "mid", "low"}},
		{SortSalaryAsc, []string{"low", "mid", "high"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.sort), func(t *testing.T) {
			page, err := testClient.Search(ctx, Query{Sort: tt.sort})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if got := jobIDs(page); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Search(sort=%s) ids = %v, want %v", tt.sort, got, tt.want)
			}
		})
	}
}

func TestPagination(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	var docs []Doc
	for i := 1; i <= 5; i++ {
		docs = append(docs, titled("j"+strconv.Itoa(i), "Engineer"))
	}
	seed(t, ctx, docs...)

	page, err := testClient.Search(ctx, Query{Sort: SortSalaryDesc, Page: 2, PerPage: 2})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if want := []string{"j3", "j4"}; !reflect.DeepEqual(jobIDs(page), want) {
		t.Errorf("page 2 ids = %v, want %v", jobIDs(page), want)
	}
	if page.Total != 5 {
		t.Errorf("Total = %d, want 5", page.Total)
	}
}

func TestSearchRejectsUnknownSort(t *testing.T) {
	if _, err := testClient.Search(testContext(t), Query{Sort: "bogus"}); err == nil {
		t.Error("Search(sort=bogus) error = nil, want error")
	}
}

func TestCountryFacetWithFilter(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	in := func(id, country string) Doc {
		d := titled(id, "Engineer "+id)
		d.Country = country
		return d
	}
	seed(t, ctx, in("ca1", "CA"), in("us1", "US"), in("us2", "US"), in("none", ""))

	page, err := testClient.Search(ctx, Query{Countries: []string{"CA"}, Sort: SortSalaryAsc})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if want := []string{"ca1"}; !reflect.DeepEqual(jobIDs(page), want) {
		t.Errorf("ids = %v, want %v", jobIDs(page), want)
	}
	want := []Facet{{Value: "US", Count: 2}, {Value: "CA", Count: 1}}
	if !reflect.DeepEqual(page.Facets, want) {
		t.Errorf("Facets = %v, want %v", page.Facets, want)
	}
}

func TestExternalVersion(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	five, three := titled("a", "Version Five"), titled("a", "Version Three")

	for _, a := range []Action{
		{Version: 5, JobID: "a", Doc: &five},
		{Version: 3, JobID: "a", Doc: &three},
	} {
		results, err := testClient.Bulk(ctx, []Action{a})
		if err != nil {
			t.Fatalf("Bulk(v%d) error = %v", a.Version, err)
		}
		if results[0].Err != nil {
			t.Errorf("Bulk(v%d) item error = %v, want nil", a.Version, results[0].Err)
		}
	}
	refresh(t, ctx)

	page, err := testClient.Search(ctx, Query{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Title != "Version Five" {
		t.Errorf("items = %+v, want the v5 doc", page.Items)
	}

	results, err := testClient.Bulk(ctx, []Action{{Version: 1, JobID: "missing"}})
	if err != nil {
		t.Fatalf("Bulk(delete missing) error = %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("delete of missing doc item error = %v, want nil", results[0].Err)
	}
}

func TestRequireAlias(t *testing.T) {
	ctx := testContext(t)
	resetIndex(t, ctx)
	t.Cleanup(func() { resetIndex(t, context.Background()) })
	if _, err := testClient.es.Indices.Delete(indexName).Do(ctx); err != nil {
		t.Fatalf("delete index: %v", err)
	}

	doc := titled("a", "Engineer")
	_, err := testClient.Bulk(ctx, []Action{{Version: 1, JobID: "a", Doc: &doc}})
	if err == nil {
		t.Fatal("Bulk() error = nil, want error for missing alias")
	}

	_, err = testClient.es.Indices.Get(aliasName).Do(ctx)
	var esErr *types.ElasticsearchError
	if !errors.As(err, &esErr) {
		t.Errorf("Bulk() auto-created an index named %s (Get error = %v)", aliasName, err)
	}
}
