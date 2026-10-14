package fx_test

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestStaticToUSD(t *testing.T) {
	// Catches: silent USD default, incorrect FX rates, and unvalidated currency codes.
	tests := []struct {
		name      string
		amount    string
		currency  posting.Currency
		wantUSD   string
		wantRate  string
		wantErrIs error
	}{
		{
			name:     "USD identity",
			amount:   "100",
			currency: posting.USD,
			wantUSD:  "100",
			wantRate: "1",
		},
		{
			name:     "CAD rate",
			amount:   "62.5",
			currency: posting.CAD,
			wantUSD:  "43.75",
			wantRate: "0.70",
		},
		{
			name:     "GBP rate",
			amount:   "80000",
			currency: posting.GBP,
			wantUSD:  "105600",
			wantRate: "1.32",
		},
		{
			name:     "EUR rate",
			amount:   "100",
			currency: posting.EUR,
			wantUSD:  "111",
			wantRate: "1.11",
		},
		{
			name:     "PLN rate",
			amount:   "100",
			currency: posting.PLN,
			wantUSD:  "25",
			wantRate: "0.25",
		},
		{
			name:      "unsupported currency JPY",
			amount:    "1000",
			currency:  posting.Currency("JPY"),
			wantErrIs: fx.ErrUnknownCurrency,
		},
		{
			name:      "unknown bogus currency XYZ",
			amount:    "100",
			currency:  posting.Currency("XYZ"),
			wantErrIs: fx.ErrUnknownCurrency,
		},
	}

	table := fx.Default

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			amt, err := decimal.NewFromString(tc.amount)
			if err != nil {
				t.Fatalf("invalid test decimal amount %q: %v", tc.amount, err)
			}

			usd, rate, err := table.ToUSD(amt, tc.currency)
			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("ToUSD(%s, %s) error = %v, want errors.Is %v", tc.amount, tc.currency, err, tc.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("ToUSD(%s, %s) unexpected error: %v", tc.amount, tc.currency, err)
			}

			wantUSD := decimal.RequireFromString(tc.wantUSD)
			if !usd.Equal(wantUSD) {
				t.Errorf("ToUSD(%s, %s) usd = %s, want %s", tc.amount, tc.currency, usd, wantUSD)
			}

			wantRate := decimal.RequireFromString(tc.wantRate)
			if !rate.Equal(wantRate) {
				t.Errorf("ToUSD(%s, %s) rate = %s, want %s", tc.amount, tc.currency, rate, wantRate)
			}
		})
	}
}
