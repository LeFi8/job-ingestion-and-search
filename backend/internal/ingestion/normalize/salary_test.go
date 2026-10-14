package normalize

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestNormalizeSalary(t *testing.T) {
	t.Parallel()

	us := &posting.Location{Country: "US"}
	ca := &posting.Location{Country: "CA"}
	gb := &posting.Location{Country: "GB"}

	type wantSalary struct {
		amount                   string
		currency                 posting.Currency
		unit                     posting.PayUnit
		currencyGuess, unitGuess bool
	}
	tests := []struct {
		name       string
		in         raw.Salary
		loc        *posting.Location
		want       *wantSalary
		wantIssues []posting.IssueCode
	}{
		{"annual guess with currency", raw.Salary{Value: "145000", Currency: "USD"}, us, &wantSalary{"145000", "USD", posting.Annual, false, true}, nil},
		{"explicit hourly", raw.Salary{Value: "65", Currency: "USD", Unit: "hourly"}, us, &wantSalary{"65", "USD", posting.Hourly, false, false}, nil},
		{"unit alias hr", raw.Salary{Value: "65", Currency: "USD", Unit: "HR"}, us, &wantSalary{"65", "USD", posting.Hourly, false, false}, nil},
		{"unit alias yearly", raw.Salary{Value: "65", Currency: "USD", Unit: " yearly "}, us, &wantSalary{"65", "USD", posting.Annual, false, false}, nil},
		{"bare number US", raw.Salary{Value: "150000"}, us, &wantSalary{"150000", "USD", posting.Annual, true, true}, nil},
		{"bare number CA", raw.Salary{Value: "62.5"}, ca, &wantSalary{"62.5", "CAD", posting.Hourly, true, true}, nil},
		{"bare number GB", raw.Salary{Value: "80000"}, gb, &wantSalary{"80000", "GBP", posting.Annual, true, true}, nil},
		{"currency trimmed and uppercased", raw.Salary{Value: "100", Currency: " usd "}, us, &wantSalary{"100", "USD", posting.Hourly, false, true}, nil},
		{"just under 1000", raw.Salary{Value: "999.99"}, us, &wantSalary{"999.99", "USD", posting.Hourly, true, true}, nil},
		{"1000 ambiguous", raw.Salary{Value: "1000"}, us, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous}},
		{"5000 ambiguous", raw.Salary{Value: "5000"}, us, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous}},
		{"19999.99 ambiguous", raw.Salary{Value: "19999.99"}, us, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous}},
		{"20000 annual", raw.Salary{Value: "20000"}, us, &wantSalary{"20000", "USD", posting.Annual, true, true}, nil},
		{"unknown unit", raw.Salary{Value: "5000", Currency: "USD", Unit: "monthly"}, us, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous}},
		{"no country", raw.Salary{Value: "90000"}, nil, nil, []posting.IssueCode{posting.SalaryCurrencyUnknown}},
		{"location without country", raw.Salary{Value: "90000"}, &posting.Location{City: "Springfield"}, nil, []posting.IssueCode{posting.SalaryCurrencyUnknown}},
		{"zero", raw.Salary{Value: "0"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"negative", raw.Salary{Value: "-5"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"negative zero", raw.Salary{Value: "-0"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"invalid text", raw.Salary{Invalid: `"competitive"`}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"at cap", raw.Salary{Value: "10000000"}, us, &wantSalary{"10000000", "USD", posting.Annual, true, true}, nil},
		{"above cap", raw.Salary{Value: "10000000.01"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"1e20", raw.Salary{Value: "1e20"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"huge exponent", raw.Salary{Value: "1e2000000000"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"tiny exponent", raw.Salary{Value: "1e-30"}, us, nil, []posting.IssueCode{posting.SalaryInvalid}},
		{"both unit and currency fail", raw.Salary{Value: "5000"}, nil, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous, posting.SalaryCurrencyUnknown}},
		{"NUL in currency", raw.Salary{Value: "100", Currency: "US\x00D"}, us, &wantSalary{"100", "USD", posting.Hourly, false, true}, nil},
		{"exponent notation", raw.Salary{Value: "1e3"}, us, nil, []posting.IssueCode{posting.SalaryUnitAmbiguous}},
		{"absent", raw.Salary{}, us, nil, []posting.IssueCode{posting.SalaryMissing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Catches: monthly treated as annual, a USD default, annualising hourly, a hang on huge exponents.
			start := time.Now()
			got, issues := normalizeSalary(tt.in, tt.loc)
			if d := time.Since(start); d > time.Second {
				t.Errorf("took %v, want well under a second", d)
			}
			var codes []posting.IssueCode
			for _, is := range issues {
				codes = append(codes, is.Code)
			}
			if !slices.Equal(codes, tt.wantIssues) {
				t.Errorf("issues = %v, want %v", codes, tt.wantIssues)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil salary")
			}
			if !got.Amount.Equal(decimal.RequireFromString(tt.want.amount)) {
				t.Errorf("amount = %s, want %s", got.Amount, tt.want.amount)
			}
			if got.Currency != tt.want.currency || got.Unit != tt.want.unit ||
				got.CurrencyGuess != tt.want.currencyGuess || got.UnitGuess != tt.want.unitGuess {
				t.Errorf("got %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func TestNormalizeSalaryNullAbsent(t *testing.T) {
	t.Parallel()

	// Catches: a JSON null salary being treated as invalid instead of missing.
	var p raw.Posting
	if err := json.Unmarshal([]byte(`{"salary":null}`), &p); err != nil {
		t.Fatal(err)
	}
	got, issues := normalizeSalary(p.Salary, &posting.Location{Country: "US"})
	if got != nil || len(issues) != 1 || issues[0].Code != posting.SalaryMissing {
		t.Errorf("got %+v, %v; want nil, salary_missing", got, issues)
	}
}
