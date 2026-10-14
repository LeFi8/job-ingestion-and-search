package approval

import (
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type ReasonCode string

const (
	TitleMissing          ReasonCode = "title_missing"
	NotFullTime           ReasonCode = "not_full_time"
	StaffingFirm          ReasonCode = "staffing_firm"
	LocationNotEligible   ReasonCode = "location_not_eligible"
	SalaryMissing         ReasonCode = "salary_missing"
	SalaryInvalid         ReasonCode = "salary_invalid"
	SalaryUnitAmbiguous   ReasonCode = "salary_unit_ambiguous"
	SalaryCurrencyUnknown ReasonCode = "salary_currency_unknown"
	SalaryBelow           ReasonCode = "salary_below"
	LanguageNotAllowed    ReasonCode = "language_not_allowed"
	LanguageUndetermined  ReasonCode = "language_undetermined"
)

type Reason struct {
	Code   ReasonCode `json:"code"`
	Detail string     `json:"detail"`
}

type FXApplied struct {
	Rate      decimal.Decimal
	AmountUSD decimal.Decimal
	Inferred  bool
}

type FXRates interface {
	ToUSD(amount decimal.Decimal, cur posting.Currency) (usd, rate decimal.Decimal, err error)
}

type LanguageDetector interface {
	Detect(text string) (lang posting.Language, reliable bool)
}

type Decision struct {
	Job           *Job
	Reasons       []Reason
	Market        string
	FX            *FXApplied
	PolicyVersion string
}

func (d Decision) Approved() bool { return d.Job != nil }
