package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

type Sort string

const (
	SortRelevance  Sort = "relevance"
	SortSalaryDesc Sort = "salary_desc"
	SortSalaryAsc  Sort = "salary_asc"
	SortDateDesc   Sort = "date_desc"
	SortDateAsc    Sort = "date_asc"
)

const (
	defaultPerPage = 20
	facetSize      = 50
	emOpen         = "<em>"
	emClose        = "</em>"
)

type Query struct {
	Q         string
	Countries []string
	Sort      Sort
	Page      int
	PerPage   int
}

type Segment struct {
	Text  string
	Match bool
}

type Hit struct {
	Doc
	Highlight []Segment
}

type Facet struct {
	Value string
	Count int64
}

type Page struct {
	Items  []Hit
	Total  int64
	Facets []Facet
}

func (c *Client) Search(ctx context.Context, q Query) (Page, error) {
	body, err := buildSearchBody(q)
	if err != nil {
		return Page{}, err
	}
	resp, err := c.es.Search().Index(aliasName).Raw(bytes.NewReader(body)).Do(ctx)
	if err != nil {
		return Page{}, fmt.Errorf("search: %w", err)
	}
	return parseSearchResponse(resp.Hits, resp.Aggregations)
}

func buildSearchBody(q Query) ([]byte, error) {
	text := strings.TrimSpace(q.Q)
	sort, err := sortClause(q.Sort, text != "")
	if err != nil {
		return nil, err
	}

	page, perPage := q.Page, q.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = defaultPerPage
	}

	body := map[string]any{
		"track_total_hits": true,
		"query":            map[string]any{"match_all": map[string]any{}},
		"aggs":             map[string]any{"country": map[string]any{"terms": map[string]any{"field": "country", "size": facetSize}}},
		"sort":             sort,
		"from":             (page - 1) * perPage,
		"size":             perPage,
	}
	if text != "" {
		body["query"] = textQuery(text)
		body["highlight"] = highlightClause(text)
	}
	if len(q.Countries) > 0 {
		body["post_filter"] = map[string]any{"terms": map[string]any{"country": q.Countries}}
	}

	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode search body: %w", err)
	}
	return out, nil
}

func textQuery(text string) map[string]any {
	return map[string]any{"bool": map[string]any{
		"should": []any{
			map[string]any{"multi_match": map[string]any{
				"query":  text,
				"type":   "bool_prefix",
				"fields": []string{"title.sayt", "title.sayt._2gram", "title.sayt._3gram"},
			}},
			fuzzyTitleMatch(text),
		},
		"minimum_should_match": 1,
	}}
}

func fuzzyTitleMatch(text string) map[string]any {
	return map[string]any{"match": map[string]any{"title": map[string]any{"query": text, "fuzziness": "AUTO"}}}
}

func highlightClause(text string) map[string]any {
	return map[string]any{
		"encoder": "html",
		"fields": map[string]any{"title": map[string]any{
			"number_of_fragments": 0,
			"highlight_query": map[string]any{"bool": map[string]any{"should": []any{
				map[string]any{"match_bool_prefix": map[string]any{"title": map[string]any{"query": text}}},
				fuzzyTitleMatch(text),
			}}},
		}},
	}
}

func sortClause(s Sort, hasText bool) ([]any, error) {
	if s == "" {
		s = SortDateDesc
		if hasText {
			s = SortRelevance
		}
	}
	tiebreak := map[string]any{"job_id": "asc"}
	switch s {
	case SortRelevance:
		return []any{map[string]any{"_score": "desc"}, tiebreak}, nil
	case SortSalaryDesc:
		return []any{fieldSort("salary_usd_annual", "desc"), tiebreak}, nil
	case SortSalaryAsc:
		return []any{fieldSort("salary_usd_annual", "asc"), tiebreak}, nil
	case SortDateDesc:
		return []any{fieldSort("posted_on", "desc"), tiebreak}, nil
	case SortDateAsc:
		return []any{fieldSort("posted_on", "asc"), tiebreak}, nil
	}
	return nil, fmt.Errorf("unknown sort %q", s)
}

func fieldSort(field, order string) map[string]any {
	return map[string]any{field: map[string]any{"order": order, "missing": "_last"}}
}

func parseSearchResponse(hits types.HitsMetadata, aggs map[string]types.Aggregate) (Page, error) {
	page := Page{Items: make([]Hit, 0, len(hits.Hits)), Facets: []Facet{}}
	if hits.Total != nil {
		page.Total = hits.Total.Value
	}
	for _, h := range hits.Hits {
		var hit Hit
		if err := json.Unmarshal(h.Source_, &hit.Doc); err != nil {
			return Page{}, fmt.Errorf("decode hit source: %w", err)
		}
		if fragments := h.Highlight["title"]; len(fragments) > 0 {
			hit.Highlight = splitHighlight(fragments[0])
		}
		page.Items = append(page.Items, hit)
	}

	terms, ok := aggs["country"].(*types.StringTermsAggregate)
	if !ok {
		return Page{}, fmt.Errorf("country aggregation has unexpected type %T", aggs["country"])
	}
	buckets, ok := terms.Buckets.([]types.StringTermsBucket)
	if !ok {
		return Page{}, fmt.Errorf("country buckets have unexpected type %T", terms.Buckets)
	}
	for _, b := range buckets {
		value, ok := b.Key.(string)
		if !ok {
			return Page{}, fmt.Errorf("country bucket key has unexpected type %T", b.Key)
		}
		page.Facets = append(page.Facets, Facet{Value: value, Count: b.DocCount})
	}
	return page, nil
}

func splitHighlight(fragment string) []Segment {
	var segments []Segment
	for fragment != "" {
		plain, rest, found := strings.Cut(fragment, emOpen)
		if plain != "" {
			segments = append(segments, Segment{Text: html.UnescapeString(plain)})
		}
		if !found {
			break
		}
		matched, rest, _ := strings.Cut(rest, emClose)
		if matched != "" {
			segments = append(segments, Segment{Text: html.UnescapeString(matched), Match: true})
		}
		fragment = rest
	}
	return segments
}
