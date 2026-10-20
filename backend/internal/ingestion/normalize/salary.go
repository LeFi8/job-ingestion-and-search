package normalize

import (
	"strings"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

// MaxSalaryAmount is a plausibility cap that also keeps every stored amount,
// and its × 2080 annualisation, inside numeric(14,2).
const MaxSalaryAmount = 10_000_000

const (
	// storedDecimals matches salary_amount numeric(14,2).
	storedDecimals = 2
	maxExponent    = 10
	hourlyBelow    = 1000
	annualFrom     = 20000
)

var maxSalary = decimal.NewFromInt(MaxSalaryAmount)

func normalizeSalary(rs raw.Salary, loc *posting.Location) (*posting.Salary, []posting.Issue) {
	if rs.Value == "" && rs.Invalid == "" {
		return nil, []posting.Issue{{Code: posting.SalaryMissing}}
	}
	if rs.Invalid != "" {
		return nil, []posting.Issue{invalidSalary(rs.Invalid)}
	}
	amount, err := decimal.NewFromString(rs.Value.String())
	if err != nil {
		return nil, []posting.Issue{invalidSalary(rs.Value.String())}
	}
	if exp := amount.Exponent(); exp < -maxExponent || exp > maxExponent ||
		!amount.IsPositive() || amount.GreaterThan(maxSalary) {
		return nil, []posting.Issue{invalidSalary(rs.Value.String())}
	}
	if amount = amount.Round(storedDecimals); !amount.IsPositive() {
		return nil, []posting.Issue{invalidSalary(rs.Value.String())}
	}

	var issues []posting.Issue
	unit, unitOK := resolveUnit(amount, rs.Unit)
	switch {
	case !unitOK:
		issues = append(issues, posting.Issue{Code: posting.SalaryUnitAmbiguous, Detail: cleanText(rs.Value.String())})
	case !plausibleForUnit(amount, unit):
		issues = append(issues, posting.Issue{Code: posting.SalaryUnitAmbiguous, Detail: cleanText(rs.Value.String()) + " " + string(unit)})
	}
	currency, currencyGuess, currencyOK := resolveCurrency(rs.Currency, loc)
	if !currencyOK {
		issues = append(issues, posting.Issue{Code: posting.SalaryCurrencyUnknown, Detail: string(currency)})
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return &posting.Salary{
		Amount:        amount,
		Currency:      currency,
		Unit:          unit,
		CurrencyGuess: currencyGuess,
	}, nil
}

func invalidSalary(text string) posting.Issue {
	return posting.Issue{Code: posting.SalaryInvalid, Detail: cleanText(text)}
}

func resolveUnit(amount decimal.Decimal, unit string) (posting.PayUnit, bool) {
	if unit = strings.ToLower(cleanText(unit)); unit != "" {
		switch unit {
		case "hourly", "hour", "hr", "h", "per hour":
			return posting.Hourly, true
		case "annual", "annually", "yearly", "year", "yr", "per year", "per annum":
			return posting.Annual, true
		}
		return "", false
	}
	switch {
	case amount.LessThan(decimal.NewFromInt(hourlyBelow)):
		return posting.Hourly, true
	case amount.GreaterThanOrEqual(decimal.NewFromInt(annualFrom)):
		return posting.Annual, true
	}
	return "", false
}

// plausibleForUnit rejects an explicit unit that contradicts the amount, using
// the same boundary that decides a bare amount's unit.
func plausibleForUnit(amount decimal.Decimal, unit posting.PayUnit) bool {
	if unit == posting.Hourly {
		return amount.LessThan(decimal.NewFromInt(hourlyBelow))
	}
	return amount.GreaterThanOrEqual(decimal.NewFromInt(hourlyBelow))
}

func resolveCurrency(currency string, loc *posting.Location) (posting.Currency, bool, bool) {
	if currency = strings.ToUpper(cleanText(currency)); currency != "" {
		c := posting.Currency(currency)
		return c, false, c.Known()
	}
	if loc == nil {
		return "", true, false
	}
	local, ok := loc.Country.LocalCurrency()
	return local, true, ok
}
