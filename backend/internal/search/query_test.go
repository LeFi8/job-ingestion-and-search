package search

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSplitHighlightSkipsEmptyMatch(t *testing.T) {
	got := splitHighlight("<em></em>Backend <em>Engineer</em>")
	want := []Segment{{Text: "Backend "}, {Text: "Engineer", Match: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitHighlight() = %#v, want %#v", got, want)
	}
}

func TestBuildSearchBodyBlankQueryListsByDate(t *testing.T) {
	body, err := buildSearchBody(Query{Q: "  \t"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Query     map[string]any `json:"query"`
		Highlight any            `json:"highlight"`
		Sort      []any          `json:"sort"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Query["match_all"]; !ok || len(got.Query) != 1 {
		t.Errorf("query = %v, want match_all", got.Query)
	}
	if got.Highlight != nil {
		t.Errorf("highlight = %v, want none", got.Highlight)
	}
	wantSort := []any{
		map[string]any{"posted_on": map[string]any{"order": "desc", "missing": "_last"}},
		map[string]any{"job_id": "asc"},
	}
	if !reflect.DeepEqual(got.Sort, wantSort) {
		t.Errorf("sort = %v, want %v", got.Sort, wantSort)
	}
}
