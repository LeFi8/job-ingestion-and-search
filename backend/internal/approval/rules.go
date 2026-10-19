package approval

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

// Facts is what the engine derives once per posting; every Rule reads it.
type Facts struct {
	Posting posting.Posting
	Issues  []posting.Issue
	Market  MarketPolicy
	Matched bool
	// USD is the salary in USD per Posting.Salary.Unit; zero when FX is nil.
	USD      decimal.Decimal
	FX       *FXApplied
	FXErr    error
	Language posting.Language
}

type Rule func(Facts) []Reason

func DefaultRules(excluded ...posting.CompanyType) []Rule {
	return []Rule{
		BlockingIssues(),
		TitlePresent(),
		FullTimeOnly(),
		ExcludeCompanyTypes(excluded...),
		EligibleLocation(),
		ConvertibleCurrency(),
		SalaryAboveMarketMinimum(),
		LanguageDetermined(),
		LanguageAllowedInMarket(),
	}
}

var blockingIssueReasons = map[posting.IssueCode]ReasonCode{
	posting.SalaryMissing:         SalaryMissing,
	posting.SalaryInvalid:         SalaryInvalid,
	posting.SalaryUnitAmbiguous:   SalaryUnitAmbiguous,
	posting.SalaryCurrencyUnknown: SalaryCurrencyUnknown,
}

func BlockingIssues() Rule {
	return func(f Facts) []Reason {
		var reasons []Reason
		for _, issue := range f.Issues {
			if code, ok := blockingIssueReasons[issue.Code]; ok {
				reasons = append(reasons, Reason{Code: code, Detail: issue.Detail})
			}
		}
		if f.Posting.Salary == nil && len(reasons) == 0 {
			reasons = append(reasons, Reason{Code: SalaryMissing, Detail: "missing salary"})
		}
		return reasons
	}
}

func TitlePresent() Rule {
	return func(f Facts) []Reason {
		if strings.TrimSpace(f.Posting.Title) != "" {
			return nil
		}
		return []Reason{{Code: TitleMissing, Detail: "missing title"}}
	}
}

func FullTimeOnly() Rule {
	return func(f Facts) []Reason {
		if f.Posting.EmploymentType == posting.FullTime {
			return nil
		}
		return []Reason{{Code: NotFullTime, Detail: fmt.Sprintf("employment type %s is not full_time", f.Posting.EmploymentType)}}
	}
}

func ExcludeCompanyTypes(types ...posting.CompanyType) Rule {
	return func(f Facts) []Reason {
		if !slices.Contains(types, f.Posting.CompanyType) {
			return nil
		}
		return []Reason{{Code: StaffingFirm, Detail: fmt.Sprintf("company type %s is excluded", f.Posting.CompanyType)}}
	}
}

func EligibleLocation() Rule {
	return func(f Facts) []Reason {
		if f.Matched {
			return nil
		}
		return []Reason{{Code: LocationNotEligible, Detail: "location does not match any eligible market"}}
	}
}

func ConvertibleCurrency() Rule {
	return func(f Facts) []Reason {
		if f.FXErr == nil {
			return nil
		}
		return []Reason{{Code: SalaryCurrencyUnknown, Detail: f.FXErr.Error()}}
	}
}

func SalaryAboveMarketMinimum() Rule {
	return func(f Facts) []Reason {
		if f.FX == nil {
			return nil
		}
		if f.Posting.Salary.Unit == posting.Hourly {
			if f.USD.GreaterThan(f.Market.MinHourlyUSD) {
				return nil
			}
			return []Reason{{Code: SalaryBelow, Detail: fmt.Sprintf("%s USD/h <= %s USD/h", f.USD.StringFixed(2), f.Market.MinHourlyUSD.StringFixed(2))}}
		}
		if f.USD.GreaterThan(f.Market.MinAnnualUSD) {
			return nil
		}
		return []Reason{{Code: SalaryBelow, Detail: fmt.Sprintf("%s USD <= %s USD", f.USD.StringFixed(2), f.Market.MinAnnualUSD.StringFixed(2))}}
	}
}

func LanguageDetermined() Rule {
	return func(f Facts) []Reason {
		if f.Language != "" {
			return nil
		}
		return []Reason{{Code: LanguageUndetermined, Detail: "language could not be reliably determined"}}
	}
}

func LanguageAllowedInMarket() Rule {
	return func(f Facts) []Reason {
		if f.Language == "" || slices.Contains(f.Market.Languages, f.Language) {
			return nil
		}
		return []Reason{{Code: LanguageNotAllowed, Detail: fmt.Sprintf("language %s not allowed in market %s", f.Language, f.Market.Name)}}
	}
}
