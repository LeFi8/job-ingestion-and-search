package normalize

import (
	"strings"
	"time"

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
	if d, err := civil.ParseDate(s); err == nil {
		return &d, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		d := civil.DateOf(t)
		return &d, nil
	}
	return nil, []posting.Issue{{Code: posting.DateInvalid, Detail: s}}
}

var declaredLanguages = map[string]posting.Language{
	"english": posting.English, "en": posting.English,
	"french": posting.French, "fr": posting.French,
	"german": posting.German, "de": posting.German,
	"polish": posting.Polish, "pl": posting.Polish,
	"spanish": posting.Spanish, "es": posting.Spanish,
	"italian": posting.Italian, "it": posting.Italian,
	"portuguese": posting.Portuguese, "pt": posting.Portuguese,
	"dutch": posting.Dutch, "nl": posting.Dutch,
}

func parseLanguage(s string) posting.Language {
	tag, _, _ := strings.Cut(strings.ToLower(cleanText(s)), "-")
	tag, _, _ = strings.Cut(tag, "_")
	return declaredLanguages[tag]
}
