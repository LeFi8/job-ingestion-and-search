package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bulkServer(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func twoActions() []Action {
	return []Action{
		{Version: 1, JobID: "a", Doc: &Doc{JobID: "a", Title: "A"}},
		{Version: 2, JobID: "b", Doc: &Doc{JobID: "b", Title: "B"}},
	}
}

func TestBulkResponseErrors(t *testing.T) {
	const ok = `{"index":{"_index":"jobs_v1","_id":"a","status":200}}`

	t.Run("429 item fails the call", func(t *testing.T) {
		c := bulkServer(t, `{"errors":true,"items":[`+ok+`,{"index":{"_index":"jobs_v1","_id":"b","status":429,"error":{"type":"es_rejected_execution_exception","reason":"queue full"}}}]}`)
		if _, err := c.Bulk(context.Background(), twoActions()); err == nil {
			t.Error("Bulk() error = nil, want error for a 429 item")
		}
	})

	t.Run("fewer items than actions fails the call", func(t *testing.T) {
		c := bulkServer(t, `{"errors":false,"items":[`+ok+`]}`)
		if _, err := c.Bulk(context.Background(), twoActions()); err == nil {
			t.Error("Bulk() error = nil, want error for a short response")
		}
	})

	for name, doc := range map[string]*Doc{
		"different Doc.JobID": {JobID: "other", Title: "B"},
		"empty Doc.JobID":     {Title: "B"},
	} {
		t.Run(name+" fails the call", func(t *testing.T) {
			c := bulkServer(t, `{"errors":false,"items":[`+ok+`,`+ok+`]}`)
			actions := twoActions()
			actions[1].Doc = doc
			if _, err := c.Bulk(context.Background(), actions); err == nil {
				t.Error("Bulk() error = nil, want error for mismatched job ids")
			}
		})
	}

	t.Run("400 item sets only that item error", func(t *testing.T) {
		c := bulkServer(t, `{"errors":true,"items":[`+ok+`,{"index":{"_index":"jobs_v1","_id":"b","status":400,"error":{"type":"document_parsing_exception","reason":"bad doc"}}}]}`)
		results, err := c.Bulk(context.Background(), twoActions())
		if err != nil {
			t.Fatalf("Bulk() error = %v, want nil", err)
		}
		if results[0].Err != nil {
			t.Errorf("item 0 error = %v, want nil", results[0].Err)
		}
		if results[1].Err == nil || !strings.Contains(results[1].Err.Error(), "bad doc") {
			t.Errorf("item 1 error = %v, want one mentioning the reason", results[1].Err)
		}
	})
}
