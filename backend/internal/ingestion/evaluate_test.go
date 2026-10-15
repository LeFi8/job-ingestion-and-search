package ingestion_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/langdetect"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type expectedDecision struct {
	Index    int      `json:"index"`
	Title    string   `json:"title"`
	Approved bool     `json:"approved"`
	Reasons  []string `json:"reasons"`
}

type fakeDetector map[string]struct {
	lang     posting.Language
	reliable bool
}

func (f fakeDetector) Detect(text string) (posting.Language, bool) {
	if res, ok := f[text]; ok {
		return res.lang, res.reliable
	}
	return posting.English, true
}

func loadExpectedDecisions(t *testing.T) []expectedDecision {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "expected_decisions.json"))
	if err != nil {
		t.Fatalf("read expected_decisions.json: %v", err)
	}
	var expected []expectedDecision
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("unmarshal expected_decisions.json: %v", err)
	}
	return expected
}

func TestPipelineFixture(t *testing.T) {
	expected := loadExpectedDecisions(t)

	detector := fakeDetector{
		"Rejoignez notre équipe pour concevoir des modèles de machine learning avancés.": {lang: posting.French, reliable: true},
		"Entwickeln Sie mobile Anwendungen für iOS und Android.":                         {lang: posting.German, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	f, err := os.Open(filepath.Join("testdata", "pipeline_fixture.json"))
	if err != nil {
		t.Fatalf("open pipeline_fixture.json: %v", err)
	}
	defer func() { _ = f.Close() }()

	outcomes, quarantined, err := ingestion.Evaluate(f, "test-feed", engine)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(quarantined) != 0 {
		t.Errorf("len(quarantined) = %d, want 0", len(quarantined))
	}
	if len(outcomes) != len(expected) {
		t.Fatalf("len(outcomes) = %d, want %d", len(outcomes), len(expected))
	}

	var approvedCount, rejectedCount int
	for i, o := range outcomes {
		exp := expected[i]
		if o.Index != exp.Index {
			t.Errorf("row %d: Index = %d, want %d", i, o.Index, exp.Index)
		}
		if o.Decision.Approved() != exp.Approved {
			t.Errorf("row %d (%q): Approved = %v, want %v (reasons: %+v)", i, exp.Title, o.Decision.Approved(), exp.Approved, o.Decision.Reasons)
		}

		gotReasons := make([]string, 0, len(o.Decision.Reasons))
		for _, r := range o.Decision.Reasons {
			gotReasons = append(gotReasons, string(r.Code))
		}
		sort.Strings(gotReasons)
		wantReasons := make([]string, len(exp.Reasons))
		copy(wantReasons, exp.Reasons)
		sort.Strings(wantReasons)

		if !reflect.DeepEqual(gotReasons, wantReasons) {
			t.Errorf("row %d (%q): Reasons = %v, want %v", i, exp.Title, gotReasons, wantReasons)
		}

		if o.Decision.Approved() {
			approvedCount++
		} else {
			rejectedCount++
		}
	}

	if approvedCount != 3 || rejectedCount != 2 {
		t.Errorf("got %d approved / %d rejected, want 3 / 2", approvedCount, rejectedCount)
	}

	// Market assertions
	if outcomes[0].Decision.Market != "US" {
		t.Errorf("row 0 market = %q, want US", outcomes[0].Decision.Market)
	}
	if outcomes[1].Decision.Market != "CA" {
		t.Errorf("row 1 market = %q, want CA", outcomes[1].Decision.Market)
	}
	if outcomes[2].Decision.Market != "Remote-anywhere" {
		t.Errorf("row 2 market = %q, want Remote-anywhere", outcomes[2].Decision.Market)
	}
}

func TestPipelineFixtureRealLingua(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow real lingua test in short mode")
	}

	expected := loadExpectedDecisions(t)
	detector := langdetect.New(langdetect.MinRelativeDistance)
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	f, err := os.Open(filepath.Join("testdata", "pipeline_fixture.json"))
	if err != nil {
		t.Fatalf("open pipeline_fixture.json: %v", err)
	}
	defer func() { _ = f.Close() }()

	outcomes, quarantined, err := ingestion.Evaluate(f, "test-feed", engine)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if len(quarantined) != 0 {
		t.Errorf("len(quarantined) = %d, want 0", len(quarantined))
	}
	if len(outcomes) != len(expected) {
		t.Fatalf("len(outcomes) = %d, want %d", len(outcomes), len(expected))
	}

	var approvedCount, rejectedCount int
	for i, o := range outcomes {
		exp := expected[i]
		if o.Decision.Approved() != exp.Approved {
			t.Errorf("real lingua row %d (%q): Approved = %v, want %v (reasons: %+v)", i, exp.Title, o.Decision.Approved(), exp.Approved, o.Decision.Reasons)
		}
		if o.Decision.Approved() {
			approvedCount++
		} else {
			rejectedCount++
		}
	}

	if approvedCount != 3 || rejectedCount != 2 {
		t.Errorf("real lingua: got %d approved / %d rejected, want 3 / 2", approvedCount, rejectedCount)
	}
}

func TestPipelineQuarantined(t *testing.T) {
	input := `[
		{"title":"Backend Engineer","company":"TechCo","salary":{"value":120000,"currency":"USD"},"employment_type":"Full-Time","location":{"city":"Austin","country":"USA"}},
		"invalid non-object string",
		42,
		{"title":"Frontend Engineer","company":"TechCo","salary":{"value":110000,"currency":"USD"},"employment_type":"Full-Time","location":{"city":"Austin","country":"USA"}}
	]`

	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, fakeDetector{})
	outcomes, quarantined, err := ingestion.Evaluate(strings.NewReader(input), "test-feed", engine)
	if err != nil {
		t.Fatalf("Evaluate() unexpected error = %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("len(outcomes) = %d, want 2", len(outcomes))
	}
	if len(quarantined) != 2 {
		t.Fatalf("len(quarantined) = %d, want 2", len(quarantined))
	}
	if quarantined[0].Index != 1 || quarantined[1].Index != 2 {
		t.Errorf("quarantined indices = %d, %d; want 1, 2", quarantined[0].Index, quarantined[1].Index)
	}
}

func TestPipelineInvalidJSON(t *testing.T) {
	input := `[{"title":"Truncated"`
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, fakeDetector{})
	_, _, err := ingestion.Evaluate(strings.NewReader(input), "test-feed", engine)
	if err == nil {
		t.Fatal("expected error on truncated JSON feed, got nil")
	}
}
