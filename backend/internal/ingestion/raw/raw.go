package raw

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Posting struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Company        string   `json:"company"`
	EmploymentType string   `json:"employment_type"`
	PostingDate    string   `json:"posting_date"`
	CompanyType    string   `json:"company_type"`
	Language       string   `json:"language"`
	Remote         bool     `json:"remote"`
	Location       Location `json:"location"`
	Salary         Salary   `json:"salary"`
}

// Location is either a string ("New York, NY, USA", "Remote") or an object.
// null, absent and empty all leave every field empty.
type Location struct {
	Text                 string
	City, State, Country string
}

func (l *Location) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*l = Location{}
	switch b[0] {
	case 'n':
		return nil
	case '"':
		return json.Unmarshal(b, &l.Text)
	case '{':
		var obj struct{ City, State, Country string }
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		l.City, l.State, l.Country = obj.City, obj.State, obj.Country
		return nil
	}
	return fmt.Errorf("location must be a string or object, got %s", b)
}

// Salary is a bare number or {value, currency, unit}. Value keeps the exact JSON
// number text (62.5 stays "62.5"); "" when absent or null.
type Salary struct {
	Value    json.Number
	Currency string
	Unit     string
	// Invalid holds the original bytes when the salary is present but unusable;
	// normalization rejects it as salary_invalid.
	Invalid string
}

func (s *Salary) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*s = Salary{}
	switch {
	case string(b) == "null":
	case isNumber(b):
		s.Value = json.Number(b)
	case b[0] == '{':
		var obj struct {
			Value    json.RawMessage
			Currency string
			Unit     string
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			s.Invalid = string(b)
			return nil
		}
		if len(obj.Value) == 0 || string(obj.Value) == "null" {
			return nil
		}
		if !isNumber(obj.Value) {
			s.Invalid = string(b)
			return nil
		}
		s.Value, s.Currency, s.Unit = json.Number(obj.Value), obj.Currency, obj.Unit
	default:
		s.Invalid = string(b)
	}
	return nil
}

// isNumber relies on encoding/json having already validated the token.
func isNumber(b []byte) bool {
	return len(b) > 0 && (b[0] == '-' || b[0] >= '0' && b[0] <= '9')
}
