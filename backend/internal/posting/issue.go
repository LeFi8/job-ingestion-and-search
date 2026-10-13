package posting

type IssueCode string

const (
	LocationMissing       IssueCode = "location_missing"
	CountryUnknown        IssueCode = "country_unknown"
	SalaryMissing         IssueCode = "salary_missing"
	SalaryInvalid         IssueCode = "salary_invalid"
	SalaryUnitAmbiguous   IssueCode = "salary_unit_ambiguous"
	SalaryCurrencyUnknown IssueCode = "salary_currency_unknown"
	DateInvalid           IssueCode = "date_invalid"
)

type Issue struct {
	Code   IssueCode
	Detail string
}
