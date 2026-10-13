package normalize

import (
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestNormalizeLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         raw.Location
		remote     bool
		want       *posting.Location
		wantRemote bool
	}{
		{"object", raw.Location{City: "Austin", State: "TX", Country: "USA"}, false, &posting.Location{City: "Austin", Region: "TX", Country: "US"}, false},
		{"state CA with USA is California", raw.Location{State: "CA", Country: "USA"}, false, &posting.Location{Region: "CA", Country: "US"}, false},
		{"DC", raw.Location{City: "Washington", State: "DC", Country: "USA"}, false, &posting.Location{City: "Washington", Region: "DC", Country: "US"}, false},
		{"three segments", raw.Location{Text: "New York, NY, USA"}, false, &posting.Location{City: "New York", Region: "NY", Country: "US"}, false},
		{"two segments country alias", raw.Location{Text: "London, UK"}, true, &posting.Location{City: "London", Country: "GB"}, true},
		{"remote string", raw.Location{Text: "Remote"}, false, nil, true},
		{"null", raw.Location{}, true, nil, true},
		{"object unknown state", raw.Location{City: "Dublin", State: "Leinster", Country: "Ireland"}, true, &posting.Location{City: "Dublin", Region: "Leinster", Country: "IE"}, true},
		{"single city", raw.Location{Text: "Springfield"}, false, &posting.Location{City: "Springfield"}, false},
		{"single country", raw.Location{Text: "Canada"}, false, &posting.Location{Country: "CA"}, false},
		{"city and US state", raw.Location{Text: "San Francisco, CA"}, false, &posting.Location{City: "San Francisco", Region: "CA", Country: "US"}, false},
		{"lowercase state beats Germany", raw.Location{Text: "Wilmington, de"}, false, &posting.Location{City: "Wilmington", Region: "DE", Country: "US"}, false},
		{"unknown second segment is region", raw.Location{Text: "Austin, Texas"}, false, &posting.Location{City: "Austin", Region: "Texas"}, false},
		{"last of three is country", raw.Location{Text: "Vancouver, BC, CA"}, false, &posting.Location{City: "Vancouver", Region: "BC", Country: "CA"}, false},
		{"NUL dropped and trimmed", raw.Location{City: "Aus\x00tin ", State: "TX", Country: "USA"}, false, &posting.Location{City: "Austin", Region: "TX", Country: "US"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Catches: California read as Canada, a crash on null, 2-part strings misread.
			got, gotRemote := normalizeLocation(tt.in, tt.remote)
			if gotRemote != tt.wantRemote {
				t.Errorf("remote = %v, want %v", gotRemote, tt.wantRemote)
			}
			switch {
			case got == nil && tt.want == nil:
			case got == nil || tt.want == nil:
				t.Errorf("got %+v, want %+v", got, tt.want)
			case *got != *tt.want:
				t.Errorf("got %+v, want %+v", *got, *tt.want)
			}
		})
	}
}
