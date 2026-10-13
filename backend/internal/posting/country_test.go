package posting_test

import (
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestLookupCountry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     string
		want   posting.Country
		wantOK bool
	}{
		{"USA", "US", true},
		{" usa ", "US", true},
		{"United States", "US", true},
		{"United States of America", "US", true},
		{"Canada", "CA", true},
		{"CA", "CA", true},
		{"UK", "GB", true},
		{"GB", "GB", true},
		{"United Kingdom", "GB", true},
		{"Great Britain", "GB", true},
		{"England", "GB", true},
		{"Germany", "DE", true},
		{"ie", "IE", true},
		{"Ireland", "IE", true},
		{"France", "FR", true},
		{"Poland", "PL", true},
		{" pl ", "PL", true},
		{"Leinster", "", false},
		{"", "", false},
		{"Narnia", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, ok := posting.LookupCountry(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("LookupCountry(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestCountryLocalCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		country posting.Country
		want    posting.Currency
		wantOK  bool
	}{
		{"US", posting.USD, true},
		{"CA", posting.CAD, true},
		{"GB", posting.GBP, true},
		{"DE", posting.EUR, true},
		{"IE", posting.EUR, true},
		{"FR", posting.EUR, true},
		{"PL", posting.PLN, true},
		{"ZZ", "", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.country), func(t *testing.T) {
			t.Parallel()
			got, ok := tt.country.LocalCurrency()
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Country(%q).LocalCurrency() = (%q, %v), want (%q, %v)", tt.country, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
