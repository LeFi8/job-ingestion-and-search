package posting_test

import (
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestParseEmploymentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want posting.EmploymentType
	}{
		{"Full-Time", posting.FullTime},
		{"full time", posting.FullTime},
		{"FULLTIME", posting.FullTime},
		{"Part-Time", posting.PartTime},
		{"Contract", posting.Contract},
		{"Internship", posting.Internship},
		{"Temporary", posting.Temporary},
		{"", posting.EmploymentUnknown},
		{"gig", posting.EmploymentUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := posting.ParseEmploymentType(tt.in); got != tt.want {
				t.Errorf("ParseEmploymentType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseCompanyType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want posting.CompanyType
	}{
		{"Direct Employer", posting.DirectEmployer},
		{"staffing firm", posting.StaffingFirm},
		{"Staffing-Firm", posting.StaffingFirm},
		{"Consulting-Agency", posting.ConsultingAgency},
		{"", posting.CompanyUnknown},
		{"startup", posting.CompanyUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := posting.ParseCompanyType(tt.in); got != tt.want {
				t.Errorf("ParseCompanyType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
