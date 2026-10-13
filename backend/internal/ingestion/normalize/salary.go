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
	maxExponent = 10
	hourlyBelow = 1000
	annualFrom  = 20000
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

	var issues []posting.Issue
	unit, unitGuess, unitOK := resolveUnit(amount, rs.Unit)
	if !unitOK {
		issues = append(issues, posting.Issue{Code: posting.SalaryUnitAmbiguous, Detail: cleanText(rs.Value.String())})
	}
	currency, currencyGuess, currencyOK := resolveCurrency(rs.Currency, loc)
	if !currencyOK {
		issues = append(issues, posting.Issue{Code: posting.SalaryCurrencyUnknown})
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return &posting.Salary{
		Amount:        amount,
		Currency:      currency,
		Unit:          unit,
		CurrencyGuess: currencyGuess,
		UnitGuess:     unitGuess,
	}, nil
}

func invalidSalary(text string) posting.Issue {
	return posting.Issue{Code: posting.SalaryInvalid, Detail: cleanText(text)}
}

func resolveUnit(amount decimal.Decimal, unit string) (posting.PayUnit, bool, bool) {
	if unit = strings.ToLower(cleanText(unit)); unit != "" {
		switch unit {
		case "hourly", "hour", "hr":
			return posting.Hourly, false, true
		case "annual", "yearly", "year":
			return posting.Annual, false, true
		}
		return "", false, false
	}
	switch {
	case amount.LessThan(decimal.NewFromInt(hourlyBelow)):
		return posting.Hourly, true, true
	case amount.GreaterThanOrEqual(decimal.NewFromInt(annualFrom)):
		return posting.Annual, true, true
	}
	return "", true, false
}

func resolveCurrency(currency string, loc *posting.Location) (posting.Currency, bool, bool) {
	if currency = strings.ToUpper(cleanText(currency)); currency != "" {
		return posting.Currency(currency), false, true
	}
	if loc == nil {
		return "", true, false
	}
	local, ok := loc.Country.LocalCurrency()
	return local, true, ok
}
