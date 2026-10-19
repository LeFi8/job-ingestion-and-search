package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

// keyVersion changes every key, and so does any change to normalization the key
// reads (country aliases, US states, location segments, date parsing); either one duplicates
// stored postings unless existing rows are re-keyed in the same change.
const keyVersion = "k1"

// idempotencyKey uses normalized values where normalization succeeded and the
// cleaned raw value where it failed, so a failed lookup never merges postings.
func idempotencyKey(sourceID string, rp raw.Posting, p posting.Posting) (string, error) {
	city, region, country := keyLocation(rp.Location, p.Location)
	date := canon(rp.PostingDate)
	if p.PostedOn != nil {
		date = p.PostedOn.String()
	}
	parts, err := json.Marshal([]string{
		keyVersion, sourceID,
		canon(rp.Company), canon(rp.Title),
		city, region, country,
		date,
	})
	if err != nil {
		return "", fmt.Errorf("marshal idempotency key parts: %w", err)
	}
	sum := sha256.Sum256(parts)
	return hex.EncodeToString(sum[:]), nil
}

func keyLocation(rl raw.Location, loc *posting.Location) (city, region, country string) {
	if loc != nil {
		city, region, country = canon(loc.City), canon(loc.Region), canon(string(loc.Country))
	}
	if country == "" {
		country = canon(rawCountry(rl))
	}
	return city, region, country
}

func rawCountry(rl raw.Location) string {
	if rl.Text == "" {
		return rl.Country
	}
	segments := strings.Split(rl.Text, ",")
	if len(segments) < 3 {
		return ""
	}
	return segments[len(segments)-1]
}

func canon(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(s, "\x00", "")), " "))
}
