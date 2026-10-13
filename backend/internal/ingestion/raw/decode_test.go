package raw

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecodeEveryInputShape(t *testing.T) {
	t.Parallel()

	input := `[
		{"title":"A","location":{"city":"Austin","state":"TX","country":"USA"},"salary":{"value":145000,"currency":"USD"}},
		{"title":"B","location":"New York, NY, USA","salary":{"value":65,"currency":"USD","unit":"hourly"}},
		{"title":"C","location":"London, UK","salary":150000},
		{"title":"D","location":"Remote","remote":true,"salary":62.5},
		{"title":"","location":null,"posting_date":"","language":""}
	]`
	recs, quar, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Catches: a supported input shape being dropped or quarantined.
	if len(recs) != 5 || len(quar) != 0 {
		t.Fatalf("got %d records, %d quarantined, want 5, 0", len(recs), len(quar))
	}
	for i, r := range recs {
		if r.Index != i {
			t.Errorf("recs[%d].Index = %d", i, r.Index)
		}
	}
	if got := recs[0].Posting.Location; got != (Location{City: "Austin", State: "TX", Country: "USA"}) {
		t.Errorf("recs[0].Location = %+v", got)
	}
	if got := recs[1].Posting.Salary; got.Unit != "hourly" || got.Value != "65" {
		t.Errorf("recs[1].Salary = %+v", got)
	}
	if got := recs[2].Posting.Location.Text; got != "London, UK" {
		t.Errorf("recs[2].Location.Text = %q", got)
	}
	if got := recs[3].Posting.Salary.Value; got != "62.5" {
		t.Errorf("recs[3].Salary.Value = %q", got)
	}
	if got := recs[4].Posting; got.Title != "" || got.Location != (Location{}) {
		t.Errorf("recs[4] = %+v", got)
	}
}

func TestDecodeQuarantinesBadRecords(t *testing.T) {
	t.Parallel()

	elems := []string{
		`{"title":"ok"}`,
		`"just a string"`,
		`[1, 2]`,
		`null`,
		`{"title": 5}`,
		`{"location": 42}`,
		`{"title":"ok2"}`,
	}
	recs, quar, err := Decode(strings.NewReader("[" + strings.Join(elems, ",") + "]"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Catches: one bad record sinking the batch, or null decoding as an empty posting.
	if len(recs) != 2 || recs[0].Index != 0 || recs[1].Index != 6 {
		t.Fatalf("records = %+v, want indices 0 and 6", recs)
	}
	wantIdx := []int{1, 2, 3, 4, 5}
	if len(quar) != len(wantIdx) {
		t.Fatalf("got %d quarantined, want %d", len(quar), len(wantIdx))
	}
	for i, q := range quar {
		if q.Index != wantIdx[i] {
			t.Errorf("quar[%d].Index = %d, want %d", i, q.Index, wantIdx[i])
		}
		if q.Err == nil {
			t.Errorf("quar[%d].Err is nil", i)
		}
		if string(q.Raw) != elems[q.Index] {
			t.Errorf("quar[%d].Raw = %s, want %s", i, q.Raw, elems[q.Index])
		}
	}
	if !errors.Is(quar[2].Err, errNotObject) {
		t.Errorf("null quarantine err = %v, want errNotObject", quar[2].Err)
	}
}

func TestDecodeInvalidSalaryStillIngests(t *testing.T) {
	t.Parallel()

	recs, quar, err := Decode(strings.NewReader(`[{"title":"a","salary":"competitive"}]`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Catches: a non-numeric salary quarantining a record that normalization should handle.
	if len(recs) != 1 || len(quar) != 0 {
		t.Fatalf("got %d records, %d quarantined, want 1, 0", len(recs), len(quar))
	}
	if recs[0].Posting.Salary.Invalid == "" {
		t.Error("Salary.Invalid not set")
	}
}

func TestDecodeBrokenFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"truncated", `[{"title":"a"}, {"title":`},
		{"not an array", `{"title":"a"}`},
		{"null", `null`},
		{"empty", ``},
		{"trailing value", `[{"title":"a"}] junk`},
		{"trailing object", `[{"title":"a"}]{}`},
		{"stray bracket", `[{"title":"a"}]]`},
		{"stray brace", `[{"title":"a"}] }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recs, quar, err := Decode(strings.NewReader(tt.input))
			// Catches: a malformed or non-array file returning partial results with no error.
			if err == nil {
				t.Fatal("expected error")
			}
			if recs != nil || quar != nil {
				t.Errorf("got recs=%v quar=%v, want nil", recs, quar)
			}
		})
	}
}

func TestDecodeErrorsKeepGoCause(t *testing.T) {
	t.Parallel()

	_, _, err := Decode(strings.NewReader(`[{"title":"a"} {"title":"b"}]`))
	var syn *json.SyntaxError
	// Catches: losing the syntax error offset by flattening the error.
	if !errors.As(err, &syn) {
		t.Errorf("malformed: err = %v, want *json.SyntaxError", err)
	}

	_, _, err = Decode(strings.NewReader(`[{"title":"a"}, {"title":`))
	// Catches: a truncated file's cause being hidden (encoding/json reports it as io.ErrUnexpectedEOF, not a SyntaxError).
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated: err = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestDecodeAcceptsWhitespaceAndEmptyArray(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"[]", "[]\n\n  \t", "  [{\"title\":\"a\"}]\n"} {
		recs, quar, err := Decode(strings.NewReader(in))
		// Catches: trailing whitespace or an empty array being rejected.
		if err != nil {
			t.Errorf("Decode(%q): %v", in, err)
		}
		if len(quar) != 0 {
			t.Errorf("Decode(%q): quarantined %d", in, len(quar))
		}
		_ = recs
	}
}
