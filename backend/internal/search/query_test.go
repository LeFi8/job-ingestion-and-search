package search

import (
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
