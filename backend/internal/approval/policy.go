package approval

import (
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

const PolicyVersion = "2026-10-13.1"

type MarketPolicy struct {
	Name         string
	Matches      func(p posting.Posting) bool
	MinAnnualUSD decimal.Decimal
	MinHourlyUSD decimal.Decimal
	Languages    []posting.Language
}

type Policy struct {
	Version       string
	StaffingTypes map[posting.CompanyType]bool
	Markets       []MarketPolicy
	Fallback      MarketPolicy
}

func InCountry(c posting.Country) func(p posting.Posting) bool {
	return func(p posting.Posting) bool {
		return p.Location != nil && p.Location.Country == c
	}
}

func IsRemote(p posting.Posting) bool {
	return p.Remote
}

func RemoteIn(c posting.Country) func(p posting.Posting) bool {
	return func(p posting.Posting) bool {
		return p.Remote && p.Location != nil && p.Location.Country == c
	}
}

func DefaultPolicy() Policy {
	return Policy{
		Version: PolicyVersion,
		StaffingTypes: map[posting.CompanyType]bool{
			posting.StaffingFirm: true,
		},
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
			{
				Name:         "Remote-anywhere",
				Matches:      IsRemote,
				MinAnnualUSD: decimal.RequireFromString("100000"),
				MinHourlyUSD: decimal.RequireFromString("45"),
				Languages:    []posting.Language{posting.English},
			},
		},
		Fallback: MarketPolicy{
			Name:         "none",
			MinAnnualUSD: decimal.RequireFromString("100000"),
			MinHourlyUSD: decimal.RequireFromString("45"),
			Languages:    []posting.Language{posting.English},
		},
	}
}
