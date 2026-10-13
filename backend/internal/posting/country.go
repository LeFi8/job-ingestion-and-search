package posting

import "strings"

type Country string // "" when unknown

var countryAliases = map[string]Country{
	"us":                       "US",
	"usa":                      "US",
	"united states":            "US",
	"united states of america": "US",
	"ca":                       "CA",
	"canada":                   "CA",
	"uk":                       "GB",
	"gb":                       "GB",
	"united kingdom":           "GB",
	"great britain":            "GB",
	"england":                  "GB",
	"de":                       "DE",
	"germany":                  "DE",
	"ie":                       "IE",
	"ireland":                  "IE",
	"fr":                       "FR",
	"france":                   "FR",
	"pl":                       "PL",
	"poland":                   "PL",
}

var localCurrencies = map[Country]Currency{
	"US": USD,
	"CA": CAD,
	"GB": GBP,
	"DE": EUR,
	"IE": EUR,
	"FR": EUR,
	"PL": PLN,
}

func LookupCountry(token string) (Country, bool) {
	c, ok := countryAliases[strings.ToLower(strings.TrimSpace(token))]
	return c, ok
}

func (c Country) LocalCurrency() (Currency, bool) {
	cur, ok := localCurrencies[c]
	return cur, ok
}
