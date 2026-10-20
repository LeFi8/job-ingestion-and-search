package search

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestCreateIndexRequestKeepsEmbeddedMapping(t *testing.T) {
	req, err := createIndexRequest()
	if err != nil {
		t.Fatal(err)
	}
	sent, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(sent, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mappingJSON, &want); err != nil {
		t.Fatal(err)
	}
	want["aliases"] = map[string]any{aliasName: map[string]any{}}
	if !reflect.DeepEqual(stringLeaves(got), stringLeaves(want)) {
		t.Errorf("create index body = %s\nwant mapping.json plus the %q alias", sent, aliasName)
	}
}

// ES accepts index settings as numbers or strings; the typed client sends strings.
func stringLeaves(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = stringLeaves(e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = stringLeaves(e)
		}
		return out
	default:
		return fmt.Sprint(v)
	}
}
