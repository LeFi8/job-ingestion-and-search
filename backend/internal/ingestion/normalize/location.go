package normalize

import (
	"strings"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

var usStates = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true, "CT": true, "DE": true,
	"FL": true, "GA": true, "HI": true, "ID": true, "IL": true, "IN": true, "IA": true, "KS": true,
	"KY": true, "LA": true, "ME": true, "MD": true, "MA": true, "MI": true, "MN": true, "MS": true,
	"MO": true, "MT": true, "NE": true, "NV": true, "NH": true, "NJ": true, "NM": true, "NY": true,
	"NC": true, "ND": true, "OH": true, "OK": true, "OR": true, "PA": true, "RI": true, "SC": true,
	"SD": true, "TN": true, "TX": true, "UT": true, "VT": true, "VA": true, "WA": true, "WV": true,
	"WI": true, "WY": true, "DC": true,
}

func normalizeLocation(rl raw.Location, remote bool) (*posting.Location, bool) {
	var loc posting.Location
	if rl.Text != "" {
		if strings.EqualFold(cleanText(rl.Text), "remote") {
			return nil, true
		}
		var textRemote bool
		loc, textRemote = parseLocationText(rl.Text)
		remote = remote || textRemote
	} else {
		loc = posting.Location{City: cleanText(rl.City), Region: cleanText(rl.State)}
		loc.Country, _ = posting.LookupCountry(cleanText(rl.Country))
	}
	if loc == (posting.Location{}) {
		return nil, remote
	}
	return &loc, remote
}

func parseLocationText(text string) (posting.Location, bool) {
	segments := strings.Split(text, ",")
	for i, s := range segments {
		segments[i] = cleanText(s)
	}
	remote := false
	if len(segments) > 1 {
		switch {
		case strings.EqualFold(segments[0], "remote"):
			segments, remote = segments[1:], true
		case strings.EqualFold(segments[len(segments)-1], "remote"):
			segments, remote = segments[:len(segments)-1], true
		}
	}
	if remote && len(segments) == 1 && usStates[strings.ToUpper(segments[0])] {
		return posting.Location{Region: strings.ToUpper(segments[0]), Country: "US"}, true
	}
	return parseLocationSegments(segments), remote
}

func parseLocationSegments(segments []string) posting.Location {
	switch len(segments) {
	case 1:
		if c, ok := posting.LookupCountry(segments[0]); ok {
			return posting.Location{Country: c}
		}
		return posting.Location{City: segments[0]}
	case 2:
		city, last := segments[0], segments[1]
		if usStates[strings.ToUpper(last)] {
			return posting.Location{City: city, Region: strings.ToUpper(last), Country: "US"}
		}
		if c, ok := posting.LookupCountry(last); ok {
			return posting.Location{City: city, Country: c}
		}
		return posting.Location{City: city, Region: last}
	}
	region := segments[1]
	if len(region) == 2 {
		region = strings.ToUpper(region)
	}
	c, _ := posting.LookupCountry(segments[len(segments)-1])
	return posting.Location{City: segments[0], Region: region, Country: c}
}
