package raw

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errNotObject = errors.New("element is not a JSON object")

type Record struct {
	Index   int
	Posting Posting
	Raw     json.RawMessage
}

type Quarantined struct {
	Index int
	Raw   json.RawMessage
	Err   error
}

// Decode reads a JSON array of postings. A record that can't be decoded is
// quarantined and the rest continue; a file that isn't a single valid JSON array fails.
func Decode(r io.Reader) ([]Record, []Quarantined, error) {
	dec := json.NewDecoder(r)
	var elems []json.RawMessage
	if err := dec.Decode(&elems); err != nil {
		return nil, nil, fmt.Errorf("decode feed: %w", err)
	}
	if elems == nil {
		return nil, nil, errors.New("feed is not a JSON array")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, nil, errors.New("unexpected data after the JSON array")
	}

	var recs []Record
	var quarantined []Quarantined
	for i, el := range elems {
		var p Posting
		err := errNotObject
		if el[0] == '{' {
			err = json.Unmarshal(el, &p)
		}
		if err != nil {
			quarantined = append(quarantined, Quarantined{Index: i, Raw: el, Err: err})
			continue
		}
		recs = append(recs, Record{Index: i, Posting: p, Raw: el})
	}
	return recs, quarantined, nil
}
