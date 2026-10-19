package approval

import (
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

const PolicyVersion = "2026-10-19.1"

type MarketPolicy struct {
	Name         string
	Matches      func(p posting.Posting) bool
	MinAnnualUSD decimal.Decimal
	MinHourlyUSD decimal.Decimal
	Languages    []posting.Language
}

type Policy struct {
	Version string
	Rules   []Rule
	Markets []MarketPolicy
	// RemoteAnywhere applies to a remote job that no entry in Markets matched.
	RemoteAnywhere *MarketPolicy
	Fallback       MarketPolicy
}

func InCountry(c posting.Country) func(p posting.Posting) bool {
	return func(p posting.Posting) bool {
		return p.Location != nil && p.Location.Country == c
	}
}

func RemoteIn(c posting.Country) func(p posting.Posting) bool {
	return func(p posting.Posting) bool {
		return p.Remote && p.Location != nil && p.Location.Country == c
	}
}

func DefaultPolicy() Policy {
	return Policy{
		Version: PolicyVersion,
		Rules:   DefaultRules(posting.StaffingFirm),
		Markets: []MarketPolicy{
			{
				Name:         "US",
				Matches:      InCountry(posting.Country("US")),
				MinAnnualUSD: decimal.RequireFromString("100000"),
				MinHourlyUSD: decimal.RequireFromString("45"),
				Languages:    []posting.Language{posting.English},
			},
			{
				Name:         "CA",
				Matches:      InCountry(posting.Country("CA")),
				MinAnnualUSD: decimal.RequireFromString("100000"),
				MinHourlyUSD: decimal.RequireFromString("45"),
				Languages:    []posting.Language{posting.English, posting.French},
			},
		},
		RemoteAnywhere: &MarketPolicy{
			Name:         "Remote-anywhere",
			MinAnnualUSD: decimal.RequireFromString("100000"),
			MinHourlyUSD: decimal.RequireFromString("45"),
			Languages:    []posting.Language{posting.English},
		},
		Fallback: MarketPolicy{
			Name:         "none",
			MinAnnualUSD: decimal.RequireFromString("100000"),
			MinHourlyUSD: decimal.RequireFromString("45"),
			Languages:    []posting.Language{posting.English},
		},
	}
}
