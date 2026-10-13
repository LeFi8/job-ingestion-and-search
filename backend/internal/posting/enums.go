package posting

import "strings"

type EmploymentType string

const (
	FullTime          EmploymentType = "full_time"
	PartTime          EmploymentType = "part_time"
	Contract          EmploymentType = "contract"
	Internship        EmploymentType = "internship"
	Temporary         EmploymentType = "temporary"
	EmploymentUnknown EmploymentType = "unknown"
)

type CompanyType string

const (
	DirectEmployer   CompanyType = "direct_employer"
	StaffingFirm     CompanyType = "staffing_firm"
	ConsultingAgency CompanyType = "consulting_agency"
	CompanyUnknown   CompanyType = "unknown"
)

type PayUnit string

const (
	Hourly PayUnit = "hourly"
	Annual PayUnit = "annual"
)

var squeezeSeparators = strings.NewReplacer(" ", "", "-", "", "_", "")

func normalizeEnum(s string) string {
	return squeezeSeparators.Replace(strings.ToLower(strings.TrimSpace(s)))
}

func ParseEmploymentType(s string) EmploymentType {
	switch normalizeEnum(s) {
	case "fulltime":
		return FullTime
	case "parttime":
		return PartTime
	case "contract":
		return Contract
	case "internship":
		return Internship
	case "temporary":
		return Temporary
	}
	return EmploymentUnknown
}

func ParseCompanyType(s string) CompanyType {
	switch normalizeEnum(s) {
	case "directemployer":
		return DirectEmployer
	case "staffingfirm":
		return StaffingFirm
	case "consultingagency":
		return ConsultingAgency
	}
	return CompanyUnknown
}
