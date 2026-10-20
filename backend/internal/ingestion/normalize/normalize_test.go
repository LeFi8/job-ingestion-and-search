package normalize

import (
	"testing"

	"cloud.google.com/go/civil"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	date := civil.Date{Year: 2023, Month: 10, Day: 3}
	tests := []struct {
		name       string
		in         raw.Posting
		check      func(*testing.T, posting.Posting)
		wantIssues []posting.IssueCode
	}{
		{"date", raw.Posting{PostingDate: "2023-10-03"}, func(t *testing.T, p posting.Posting) {
			if p.PostedOn == nil || *p.PostedOn != date {
				t.Errorf("PostedOn = %v, want %v", p.PostedOn, date)
			}
		}, []posting.IssueCode{posting.SalaryMissing}},
		{"date trimmed", raw.Posting{PostingDate: " 2023-10-03 "}, func(t *testing.T, p posting.Posting) {
			if p.PostedOn == nil || *p.PostedOn != date {
				t.Errorf("PostedOn = %v, want %v", p.PostedOn, date)
			}
		}, []posting.IssueCode{posting.SalaryMissing}},
		{"date empty", raw.Posting{}, func(t *testing.T, p posting.Posting) {
			if p.PostedOn != nil {
				t.Errorf("PostedOn = %v, want nil", p.PostedOn)
			}
		}, []posting.IssueCode{posting.SalaryMissing}},
		{"date words", raw.Posting{PostingDate: "yesterday"}, func(t *testing.T, p posting.Posting) {
			if p.PostedOn != nil {
				t.Errorf("PostedOn = %v, want nil", p.PostedOn)
			}
		}, []posting.IssueCode{posting.SalaryMissing, posting.DateInvalid}},
		{"date impossible", raw.Posting{PostingDate: "2023-02-30"}, func(t *testing.T, p posting.Posting) {
			if p.PostedOn != nil {
				t.Errorf("PostedOn = %v, want nil", p.PostedOn)
			}
		}, []posting.IssueCode{posting.SalaryMissing, posting.DateInvalid}},
		{"date RFC3339 UTC", raw.Posting{PostingDate: "2023-10-01T12:00:00Z"}, wantDate(civil.Date{Year: 2023, Month: 10, Day: 1}), []posting.IssueCode{posting.SalaryMissing}},
		{"date RFC3339 keeps its own offset", raw.Posting{PostingDate: "2023-10-01T23:30:00-05:00"}, wantDate(civil.Date{Year: 2023, Month: 10, Day: 1}), []posting.IssueCode{posting.SalaryMissing}},
		{"date US format", raw.Posting{PostingDate: "10/01/2023"}, wantDate(civil.Date{}), []posting.IssueCode{posting.SalaryMissing, posting.DateInvalid}},
		{"language Spanish", raw.Posting{Language: "Spanish"}, wantLanguage(posting.Spanish), []posting.IssueCode{posting.SalaryMissing}},
		{"language es", raw.Posting{Language: "es"}, wantLanguage(posting.Spanish), []posting.IssueCode{posting.SalaryMissing}},
		{"language region subtag", raw.Posting{Language: "en-US"}, wantLanguage(posting.English), []posting.IssueCode{posting.SalaryMissing}},
		{"language underscore subtag", raw.Posting{Language: "pt_BR"}, wantLanguage(posting.Portuguese), []posting.IssueCode{posting.SalaryMissing}},
		{"language Dutch", raw.Posting{Language: "Dutch"}, wantLanguage(posting.Dutch), []posting.IssueCode{posting.SalaryMissing}},
		{"language Italian", raw.Posting{Language: "it"}, wantLanguage(posting.Italian), []posting.IssueCode{posting.SalaryMissing}},
		{"language English", raw.Posting{Language: "English"}, wantLanguage(posting.English), []posting.IssueCode{posting.SalaryMissing}},
		{"language code case-insensitive", raw.Posting{Language: " EN "}, wantLanguage(posting.English), []posting.IssueCode{posting.SalaryMissing}},
		{"language French", raw.Posting{Language: "French"}, wantLanguage(posting.French), []posting.IssueCode{posting.SalaryMissing}},
		{"language German", raw.Posting{Language: "de"}, wantLanguage(posting.German), []posting.IssueCode{posting.SalaryMissing}},
		{"language Polish", raw.Posting{Language: "Polish"}, wantLanguage(posting.Polish), []posting.IssueCode{posting.SalaryMissing}},
		{"language empty", raw.Posting{}, wantLanguage(""), []posting.IssueCode{posting.SalaryMissing}},
		{"language unknown", raw.Posting{Language: "Klingon"}, wantLanguage(""), []posting.IssueCode{posting.SalaryMissing}},
		{"employment type empty", raw.Posting{}, func(t *testing.T, p posting.Posting) {
			if p.EmploymentType != posting.EmploymentUnknown || p.CompanyType != posting.CompanyUnknown {
				t.Errorf("types = %q, %q; want unknown", p.EmploymentType, p.CompanyType)
			}
		}, []posting.IssueCode{posting.SalaryMissing}},
		{"text cleaned", raw.Posting{Title: "Dev\x00eloper", Description: " d\x00 ", Company: "A\x00cme "}, func(t *testing.T, p posting.Posting) {
			if p.Title != "Developer" || p.Description != "d" || p.Company != "Acme" {
				t.Errorf("got %q %q %q", p.Title, p.Description, p.Company)
			}
		}, []posting.IssueCode{posting.SalaryMissing}},
		{"remote from location string", raw.Posting{Location: raw.Location{Text: "Remote"}, Salary: raw.Salary{Value: "150000"}}, func(t *testing.T, p posting.Posting) {
			if !p.Remote || p.Location != nil {
				t.Errorf("Remote = %v, Location = %v; want true, nil", p.Remote, p.Location)
			}
		}, []posting.IssueCode{posting.SalaryCurrencyUnknown}},
		{"salary uses location currency", raw.Posting{Location: raw.Location{Text: "London, UK"}, Salary: raw.Salary{Value: "80000"}}, func(t *testing.T, p posting.Posting) {
			if p.Salary == nil || p.Salary.Currency != posting.GBP {
				t.Errorf("Salary = %+v, want GBP", p.Salary)
			}
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Catches: unparsed dates stored, "" enums leaking out, NULs reaching Postgres.
			got, issues := Normalize("src", tt.in)
			if got.SourceID != "src" {
				t.Errorf("SourceID = %q", got.SourceID)
			}
			var codes []posting.IssueCode
			for _, is := range issues {
				codes = append(codes, is.Code)
			}
			if len(codes) != len(tt.wantIssues) {
				t.Fatalf("issues = %v, want %v", codes, tt.wantIssues)
			}
			for i := range codes {
				if codes[i] != tt.wantIssues[i] {
					t.Fatalf("issues = %v, want %v", codes, tt.wantIssues)
				}
			}
			tt.check(t, got)
		})
	}
}

func wantLanguage(want posting.Language) func(*testing.T, posting.Posting) {
	return func(t *testing.T, p posting.Posting) {
		if p.DeclaredLanguage != want {
			t.Errorf("DeclaredLanguage = %q, want %q", p.DeclaredLanguage, want)
		}
	}
}

func wantDate(want civil.Date) func(*testing.T, posting.Posting) {
	return func(t *testing.T, p posting.Posting) {
		if want == (civil.Date{}) {
			if p.PostedOn != nil {
				t.Errorf("PostedOn = %v, want nil", p.PostedOn)
			}
			return
		}
		if p.PostedOn == nil || *p.PostedOn != want {
			t.Errorf("PostedOn = %v, want %v", p.PostedOn, want)
		}
	}
}
