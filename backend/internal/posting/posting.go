package posting

import (
	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
)

// The JSON names are stored in postings.normalized and read by name in
// storage/queries/postings.sql, so they must not follow Go renames.
type Location struct {
	City    string  `json:"City"`
	Region  string  `json:"Region"`
	Country Country `json:"Country"`
}

type Salary struct {
	Amount        decimal.Decimal `json:"Amount"`
	Currency      Currency        `json:"Currency"`
	Unit          PayUnit         `json:"Unit"`
	CurrencyGuess bool            `json:"CurrencyGuess"`
}

type Posting struct {
	SourceID         string         `json:"SourceID"`
	Title            string         `json:"Title"`
	Description      string         `json:"Description"`
	Company          string         `json:"Company"`
	CompanyType      CompanyType    `json:"CompanyType"`
	Location         *Location      `json:"Location"`
	Remote           bool           `json:"Remote"`
	EmploymentType   EmploymentType `json:"EmploymentType"`
	Salary           *Salary        `json:"Salary"`
	PostedOn         *civil.Date    `json:"PostedOn"`
	DeclaredLanguage Language       `json:"DeclaredLanguage"`
}
