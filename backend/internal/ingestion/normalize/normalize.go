package normalize

import (
	"strings"

	"cloud.google.com/go/civil"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func Normalize(sourceID string, rp raw.Posting) (posting.Posting, []posting.Issue) {
	loc, remote := normalizeLocation(rp.Location, rp.Remote)
	salary, issues := normalizeSalary(rp.Salary, loc)
	postedOn, dateIssues := parseDate(rp.PostingDate)

	return posting.Posting{
		SourceID:         sourceID,
		Title:            cleanText(rp.Title),
		Description:      cleanText(rp.Description),
		Company:          cleanText(rp.Company),
		CompanyType:      posting.ParseCompanyType(rp.CompanyType),
		Location:         loc,
		Remote:           remote,
		EmploymentType:   posting.ParseEmploymentType(rp.EmploymentType),
		Salary:           salary,
		PostedOn:         postedOn,
		DeclaredLanguage: parseLanguage(rp.Language),
	}, append(issues, dateIssues...)
}

// cleanText drops U+0000 because Postgres text and jsonb reject it.
func cleanText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\x00", ""))
}

func parseDate(s string) (*civil.Date, []posting.Issue) {
	s = cleanText(s)
	if s == "" {
		return nil, nil
	}
	d, err := civil.ParseDate(s)
	if err != nil {
		return nil, []posting.Issue{{Code: posting.DateInvalid, Detail: s}}
	}
	return &d, nil
}

func parseLanguage(s string) posting.Language {
	switch strings.ToLower(cleanText(s)) {
	case "english", "en":
		return posting.English
	case "french", "fr":
		return posting.French
	case "german", "de":
		return posting.German
	case "polish", "pl":
		return posting.Polish
	}
	return ""
}
