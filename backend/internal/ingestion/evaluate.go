package ingestion

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/normalize"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type Outcome struct {
	Index    int
	Raw      raw.Posting
	RawBytes json.RawMessage
	Posting  posting.Posting
	Issues   []posting.Issue
	Decision approval.Decision
}

// Evaluate is pure: raw.Decode -> normalize -> approval.
// Quarantined records are passed through; a broken feed array returns an error.
func Evaluate(r io.Reader, sourceID string, e *approval.Engine) ([]Outcome, []raw.Quarantined, error) {
	records, quarantined, err := raw.Decode(r)
	if err != nil {
		return nil, nil, fmt.Errorf("decode feed: %w", err)
	}

	outcomes := make([]Outcome, 0, len(records))
	for _, rec := range records {
		p, issues := normalize.Normalize(sourceID, rec.Posting)
		dec := e.Evaluate(p, issues)

		outcomes = append(outcomes, Outcome{
			Index:    rec.Index,
			Raw:      rec.Posting,
			RawBytes: rec.Raw,
			Posting:  p,
			Issues:   issues,
			Decision: dec,
		})
	}

	return outcomes, quarantined, nil
}
