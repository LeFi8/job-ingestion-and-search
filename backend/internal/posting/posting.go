package posting

import (
	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
)

type Location struct {
	City, Region string
	Country      Country
}

type Salary struct {
	Amount        decimal.Decimal
	Currency      Currency
	Unit          PayUnit
	CurrencyGuess bool
	UnitGuess     bool
}

type Posting struct {
	SourceID                    string
	Title, Description, Company string
	CompanyType                 CompanyType
	Location                    *Location
	Remote                      bool
	EmploymentType              EmploymentType
	Salary                      *Salary
	PostedOn                    *civil.Date
	DeclaredLanguage            Language
}
