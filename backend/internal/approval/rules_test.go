package approval_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func usMarket() approval.MarketPolicy {
	return approval.DefaultPolicy().Markets[0]
}

func baseFacts() approval.Facts {
	p := validPosting()
	return approval.Facts{
		Posting:  p,
		Market:   usMarket(),
		Matched:  true,
		USD:      decimal.RequireFromString("150000"),
		FX:       &approval.FXApplied{Rate: decimal.NewFromInt(1), AnnualUSD: decimal.RequireFromString("150000")},
		Language: posting.English,
	}
}

func TestRules(t *testing.T) {
	// Catches: a rule that stops firing, fires on valid input, or changes the
	// detail stored with a rejection.
	fallback := approval.DefaultPolicy().Fallback
	tests := []struct {
		name  string
		rule  approval.Rule
		facts func(f *approval.Facts)
		want  []approval.Reason
	}{
		{"blocking issues pass", approval.BlockingIssues(), func(*approval.Facts) {}, nil},
		{"blocking issues map salary issues", approval.BlockingIssues(), func(f *approval.Facts) {
			f.Posting.Salary = nil
			f.Issues = []posting.Issue{{Code: posting.SalaryInvalid, Detail: "abc"}, {Code: posting.DateInvalid, Detail: "x"}}
		}, []approval.Reason{{Code: approval.SalaryInvalid, Detail: "abc"}}},
		{"blocking issues: nil salary without issue", approval.BlockingIssues(), func(f *approval.Facts) {
			f.Posting.Salary = nil
		}, []approval.Reason{{Code: approval.SalaryMissing, Detail: "missing salary"}}},
		{"title present passes", approval.TitlePresent(), func(*approval.Facts) {}, nil},
		{"title blank", approval.TitlePresent(), func(f *approval.Facts) { f.Posting.Title = "  " },
			[]approval.Reason{{Code: approval.TitleMissing, Detail: "missing title"}}},
		{"full time passes", approval.FullTimeOnly(), func(*approval.Facts) {}, nil},
		{"part time", approval.FullTimeOnly(), func(f *approval.Facts) { f.Posting.EmploymentType = posting.PartTime },
			[]approval.Reason{{Code: approval.NotFullTime, Detail: "employment type part_time is not full_time"}}},
		{"direct employer passes", approval.ExcludeCompanyTypes(posting.StaffingFirm), func(*approval.Facts) {}, nil},
		{"staffing firm", approval.ExcludeCompanyTypes(posting.StaffingFirm), func(f *approval.Facts) { f.Posting.CompanyType = posting.StaffingFirm },
			[]approval.Reason{{Code: approval.StaffingFirm, Detail: "company type staffing_firm is excluded"}}},
		{"matched location passes", approval.EligibleLocation(), func(*approval.Facts) {}, nil},
		{"unmatched location", approval.EligibleLocation(), func(f *approval.Facts) { f.Matched = false; f.Market = fallback },
			[]approval.Reason{{Code: approval.LocationNotEligible, Detail: "location does not match any eligible market"}}},
		{"converted currency passes", approval.ConvertibleCurrency(), func(*approval.Facts) {}, nil},
		{"conversion error", approval.ConvertibleCurrency(), func(f *approval.Facts) { f.FX = nil; f.FXErr = errors.New("fx: unknown currency") },
			[]approval.Reason{{Code: approval.SalaryCurrencyUnknown, Detail: "fx: unknown currency"}}},
		{"annual above minimum passes", approval.SalaryAboveMarketMinimum(), func(*approval.Facts) {}, nil},
		{"annual at minimum", approval.SalaryAboveMarketMinimum(), func(f *approval.Facts) { f.USD = decimal.RequireFromString("100000") },
			[]approval.Reason{{Code: approval.SalaryBelow, Detail: "100000.00 USD <= 100000.00 USD"}}},
		{"hourly below minimum", approval.SalaryAboveMarketMinimum(), func(f *approval.Facts) {
			f.Posting.Salary.Unit = posting.Hourly
			f.USD = decimal.RequireFromString("43.75")
		}, []approval.Reason{{Code: approval.SalaryBelow, Detail: "43.75 USD/h <= 45.00 USD/h"}}},
		{"no conversion skips salary minimum", approval.SalaryAboveMarketMinimum(), func(f *approval.Facts) { f.FX = nil; f.USD = decimal.Zero }, nil},
		{"language determined passes", approval.LanguageDetermined(), func(*approval.Facts) {}, nil},
		{"language undetermined", approval.LanguageDetermined(), func(f *approval.Facts) { f.Language = "" },
			[]approval.Reason{{Code: approval.LanguageUndetermined, Detail: "language could not be reliably determined"}}},
		{"language allowed passes", approval.LanguageAllowedInMarket(), func(*approval.Facts) {}, nil},
		{"language not allowed", approval.LanguageAllowedInMarket(), func(f *approval.Facts) {
			f.Language = posting.German
			f.Market = fallback
		}, []approval.Reason{{Code: approval.LanguageNotAllowed, Detail: "language de not allowed in market none"}}},
		{"undetermined language is not also disallowed", approval.LanguageAllowedInMarket(), func(f *approval.Facts) { f.Language = "" }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := baseFacts()
			tt.facts(&f)
			if got := tt.rule(f); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rule() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
