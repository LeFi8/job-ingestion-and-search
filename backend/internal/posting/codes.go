package posting

type Currency string

const (
	USD Currency = "USD"
	CAD Currency = "CAD"
	GBP Currency = "GBP"
	EUR Currency = "EUR"
	PLN Currency = "PLN"
)

func (c Currency) Known() bool {
	switch c {
	case USD, CAD, GBP, EUR, PLN:
		return true
	}
	return false
}

type Language string

const (
	English    Language = "en"
	French     Language = "fr"
	German     Language = "de"
	Polish     Language = "pl"
	Spanish    Language = "es"
	Italian    Language = "it"
	Portuguese Language = "pt"
	Dutch      Language = "nl"
)
