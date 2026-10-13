package raw

import (
	"encoding/json"
	"testing"
)

func TestLocationUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    Location
		wantErr bool
	}{
		{"object", `{"city":"Austin","state":"TX","country":"USA"}`, Location{City: "Austin", State: "TX", Country: "USA"}, false},
		{"string", `"New York, NY, USA"`, Location{Text: "New York, NY, USA"}, false},
		{"remote string", `"Remote"`, Location{Text: "Remote"}, false},
		{"null", `null`, Location{}, false},
		{"empty city kept empty", `{"city":"","state":"CA","country":"USA"}`, Location{State: "CA", Country: "USA"}, false},
		{"number", `42`, Location{}, true},
		{"non-string field", `{"city":5}`, Location{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got Location
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLocationAbsent(t *testing.T) {
	t.Parallel()

	// Catches: an absent location field being treated as an error or non-zero value.
	var p Posting
	if err := json.Unmarshal([]byte(`{"title":"a"}`), &p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Location != (Location{}) {
		t.Errorf("Location = %+v, want zero", p.Location)
	}
}

func TestSalaryUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Salary
	}{
		{"object with currency", `{"value":145000,"currency":"USD"}`, Salary{Value: "145000", Currency: "USD"}},
		{"object with unit", `{"value":65,"currency":"USD","unit":"hourly"}`, Salary{Value: "65", Currency: "USD", Unit: "hourly"}},
		{"bare integer", `150000`, Salary{Value: "150000"}},
		{"fraction stays exact", `62.5`, Salary{Value: "62.5"}},
		{"cents stay exact", `100000.01`, Salary{Value: "100000.01"}},
		{"null", `null`, Salary{}},
		{"string", `"competitive"`, Salary{Invalid: `"competitive"`}},
		{"object with string value", `{"value":"x","currency":"USD"}`, Salary{Invalid: `{"value":"x","currency":"USD"}`}},
		{"object with non-string currency", `{"value":5,"currency":7}`, Salary{Invalid: `{"value":5,"currency":7}`}},
		{"boolean", `true`, Salary{Invalid: `true`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got Salary
			// Catches: a salary shape that is not a number or {value,...} failing the whole record instead of being flagged Invalid.
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSalaryAbsent(t *testing.T) {
	t.Parallel()

	var p Posting
	if err := json.Unmarshal([]byte(`{"title":"a"}`), &p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Salary != (Salary{}) {
		t.Errorf("Salary = %+v, want zero", p.Salary)
	}
}
