package approval_test

import (
	"errors"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/approval"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/fx"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type fakeDetector map[string]struct {
	lang     posting.Language
	reliable bool
}

func (f fakeDetector) Detect(text string) (posting.Language, bool) {
	if r, ok := f[text]; ok {
		return r.lang, r.reliable
	}
	return "", false
}

type fakeFX map[posting.Currency]decimal.Decimal

func (f fakeFX) ToUSD(amount decimal.Decimal, cur posting.Currency) (decimal.Decimal, decimal.Decimal, error) {
	rate, ok := f[cur]
	if !ok {
		return decimal.Zero, decimal.Zero, errors.New("fx: unknown currency")
	}
	return amount.Mul(rate), rate, nil
}

func validPosting() posting.Posting {
	return posting.Posting{
		SourceID:       "test-1",
		Title:          "Senior Go Engineer",
		Description:    "Building scalable cloud backend services.",
		Company:        "Acme Corp",
		CompanyType:    posting.DirectEmployer,
		EmploymentType: posting.FullTime,
		Remote:         false,
		Location: &posting.Location{
			City:    "New York",
			Region:  "NY",
			Country: posting.Country("US"),
		},
		Salary: &posting.Salary{
			Amount:   decimal.RequireFromString("150000"),
			Currency: posting.USD,
			Unit:     posting.Annual,
		},
		DeclaredLanguage: posting.English,
	}
}

func hasReason(reasons []approval.Reason, code approval.ReasonCode) bool {
	for _, r := range reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

func TestThresholdBoundaries(t *testing.T) {
	// Catches: non-strict comparisons (>= instead of >) on salary thresholds.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	tests := []struct {
		name         string
		amount       string
		unit         posting.PayUnit
		wantApproved bool
		wantReason   approval.ReasonCode
	}{
		{
			name:         "Annual exactly 100,000 fails strict >",
			amount:       "100000",
			unit:         posting.Annual,
			wantApproved: false,
			wantReason:   approval.SalaryBelow,
		},
		{
			name:         "Annual 100,000.01 passes strict >",
			amount:       "100000.01",
			unit:         posting.Annual,
			wantApproved: true,
		},
		{
			name:         "Hourly exactly 45.00 fails strict >",
			amount:       "45.00",
			unit:         posting.Hourly,
			wantApproved: false,
			wantReason:   approval.SalaryBelow,
		},
		{
			name:         "Hourly 45.01 passes strict >",
			amount:       "45.01",
			unit:         posting.Hourly,
			wantApproved: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPosting()
			p.Salary.Amount = decimal.RequireFromString(tc.amount)
			p.Salary.Unit = tc.unit

			dec := engine.Evaluate(p, nil)
			if dec.Approved() != tc.wantApproved {
				t.Fatalf("Evaluate() approved = %v, want %v (reasons: %+v)", dec.Approved(), tc.wantApproved, dec.Reasons)
			}
			if !tc.wantApproved && !hasReason(dec.Reasons, tc.wantReason) {
				t.Errorf("Evaluate() missing expected reason %v (got %+v)", tc.wantReason, dec.Reasons)
			}
		})
	}
}

func TestHourlyNotAnnualised(t *testing.T) {
	// Catches: hourly converted to annual before gate comparison (46 * 2080 = 95680 < 100k).
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	p := validPosting()
	p.Salary.Amount = decimal.RequireFromString("46")
	p.Salary.Unit = posting.Hourly

	dec := engine.Evaluate(p, nil)
	if !dec.Approved() {
		t.Fatalf("Evaluate(46 USD/h) rejected unexpectedly: %+v", dec.Reasons)
	}

	wantSortKey := decimal.RequireFromString("95680") // 46 * 2080
	if !dec.Job.AnnualUSD().Equal(wantSortKey) {
		t.Errorf("Job.AnnualUSD() = %s, want %s", dec.Job.AnnualUSD(), wantSortKey)
	}
}

func TestAllReasonsCollected(t *testing.T) {
	// Catches: short-circuiting after the first gate violation.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	p := validPosting()
	p.Title = "   "
	p.EmploymentType = posting.Contract
	p.CompanyType = posting.StaffingFirm
	p.Salary.Amount = decimal.RequireFromString("40")
	p.Salary.Unit = posting.Hourly

	dec := engine.Evaluate(p, nil)
	if dec.Approved() {
		t.Fatal("Evaluate() approved a job that violated 4 gates")
	}

	expected := []approval.ReasonCode{
		approval.TitleMissing,
		approval.NotFullTime,
		approval.StaffingFirm,
		approval.SalaryBelow,
	}

	for _, code := range expected {
		if !hasReason(dec.Reasons, code) {
			t.Errorf("missing expected reason %v in %+v", code, dec.Reasons)
		}
	}
	if len(dec.Reasons) != len(expected) {
		t.Errorf("expected %d reasons, got %d: %+v", len(expected), len(dec.Reasons), dec.Reasons)
	}
}

func TestWhitespaceTitle(t *testing.T) {
	// Catches: untrimmed title string check.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	p := validPosting()
	p.Title = "\t \n "

	dec := engine.Evaluate(p, nil)
	if !hasReason(dec.Reasons, approval.TitleMissing) {
		t.Errorf("Evaluate() did not reject whitespace title with TitleMissing: %+v", dec.Reasons)
	}
}

func TestConsultingIsNotStaffing(t *testing.T) {
	// Catches: conflating ConsultingAgency with StaffingFirm in default policy.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}

	p := validPosting()
	p.CompanyType = posting.ConsultingAgency

	defaultEngine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)
	dec := defaultEngine.Evaluate(p, nil)
	if !dec.Approved() {
		t.Fatalf("ConsultingAgency rejected under default policy: %+v", dec.Reasons)
	}

	customPolicy := approval.DefaultPolicy()
	customPolicy.StaffingTypes[posting.ConsultingAgency] = true
	strictEngine := approval.NewEngine(customPolicy, fx.Default, detector)
	decStrict := strictEngine.Evaluate(p, nil)
	if !hasReason(decStrict.Reasons, approval.StaffingFirm) {
		t.Errorf("ConsultingAgency not rejected when StaffingTypes configured to include it: %+v", decStrict.Reasons)
	}
}

func TestDescriptionBeatsDeclared(t *testing.T) {
	// Catches: checking declared language instead of detecting from description.
	detector := fakeDetector{
		"Description written in English.":        {lang: posting.English, reliable: true},
		"Buscamos un desarrollador de software.": {lang: posting.Spanish, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	t.Run("English description with blank declared is approved", func(t *testing.T) {
		p := validPosting()
		p.Description = "Description written in English."
		p.DeclaredLanguage = ""

		dec := engine.Evaluate(p, nil)
		if !dec.Approved() {
			t.Fatalf("job rejected when description is English and declared is blank: %+v", dec.Reasons)
		}
		if dec.Job.Lang() != posting.English {
			t.Errorf("Job.Lang() = %q, want %q", dec.Job.Lang(), posting.English)
		}
	})

	t.Run("Spanish description with declared English is rejected in US market", func(t *testing.T) {
		p := validPosting()
		p.Description = "Buscamos un desarrollador de software."
		p.DeclaredLanguage = posting.English

		dec := engine.Evaluate(p, nil)
		if dec.Approved() {
			t.Fatal("expected rejection for Spanish description in US market, but job was approved")
		}
		if !hasReason(dec.Reasons, approval.LanguageNotAllowed) {
			t.Errorf("expected LanguageNotAllowed, got %+v", dec.Reasons)
		}
	})
}

func TestUnreliableFallsBackToDeclared(t *testing.T) {
	// Catches: failing to fall back to declared language when detection is unreliable.
	detector := fakeDetector{
		"Short ambiguous text": {lang: "", reliable: false},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	p := validPosting()
	p.Description = "Short ambiguous text"
	p.DeclaredLanguage = posting.German

	dec := engine.Evaluate(p, nil)
	if !hasReason(dec.Reasons, approval.LanguageNotAllowed) {
		t.Errorf("expected LanguageNotAllowed when fallback German is not allowed in US market: %+v", dec.Reasons)
	}

	p.DeclaredLanguage = ""
	decUndet := engine.Evaluate(p, nil)
	if !hasReason(decUndet.Reasons, approval.LanguageUndetermined) {
		t.Errorf("expected LanguageUndetermined when detection is unreliable and declared is empty: %+v", decUndet.Reasons)
	}
}

func TestMarkets(t *testing.T) {
	// Catches: market resolution order, location eligibility, and policy extensibility.
	detector := fakeDetector{
		"English text": {lang: posting.English, reliable: true},
		"French text":  {lang: posting.French, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	t.Run("Remote anywhere with no location", func(t *testing.T) {
		p := validPosting()
		p.Location = nil
		p.Remote = true
		p.Description = "English text"

		dec := engine.Evaluate(p, nil)
		if !dec.Approved() {
			t.Fatalf("remote job with no location rejected: %+v", dec.Reasons)
		}
		if dec.Market != "Remote-anywhere" {
			t.Errorf("Market = %q, want Remote-anywhere", dec.Market)
		}
	})

	t.Run("Remote GB job approved with FX", func(t *testing.T) {
		p := validPosting()
		p.Remote = true
		p.Location = &posting.Location{City: "London", Country: posting.Country("GB")}
		p.Salary = &posting.Salary{
			Amount:   decimal.RequireFromString("85000"),
			Currency: posting.GBP,
			Unit:     posting.Annual,
		}
		p.Description = "English text"

		dec := engine.Evaluate(p, nil)
		if !dec.Approved() {
			t.Fatalf("remote GB job rejected: %+v", dec.Reasons)
		}
		if dec.FX == nil || !dec.FX.Rate.Equal(decimal.RequireFromString("1.32")) {
			t.Errorf("FX.Rate = %v, want 1.32", dec.FX)
		}
	})

	t.Run("In-person GB fails location but evaluates other rules via Fallback", func(t *testing.T) {
		p := validPosting()
		p.Remote = false
		p.Location = &posting.Location{City: "London", Country: posting.Country("GB")}
		p.Salary = &posting.Salary{
			Amount:   decimal.RequireFromString("50000"),
			Currency: posting.GBP,
			Unit:     posting.Annual,
		}
		p.Description = "Short ambiguous text"
		p.DeclaredLanguage = posting.German

		dec := engine.Evaluate(p, nil)
		if dec.Approved() {
			t.Fatal("in-person GB job approved unexpectedly")
		}
		if !hasReason(dec.Reasons, approval.LocationNotEligible) {
			t.Errorf("missing LocationNotEligible in %+v", dec.Reasons)
		}
		if !hasReason(dec.Reasons, approval.SalaryBelow) {
			t.Errorf("missing SalaryBelow via Fallback evaluation in %+v", dec.Reasons)
		}
		if !hasReason(dec.Reasons, approval.LanguageNotAllowed) {
			t.Errorf("missing LanguageNotAllowed via Fallback evaluation in %+v", dec.Reasons)
		}
	})

	t.Run("In-person Canada allows French", func(t *testing.T) {
		p := validPosting()
		p.Location = &posting.Location{City: "Montreal", Country: posting.Country("CA")}
		p.Description = "French text"

		dec := engine.Evaluate(p, nil)
		if !dec.Approved() {
			t.Fatalf("French Canadian job rejected: %+v", dec.Reasons)
		}
	})

	t.Run("In-person US rejects French", func(t *testing.T) {
		p := validPosting()
		p.Location = &posting.Location{City: "New York", Country: posting.Country("US")}
		p.Description = "French text"

		dec := engine.Evaluate(p, nil)
		if hasReason(dec.Reasons, approval.LocationNotEligible) {
			t.Errorf("US job unexpectedly marked LocationNotEligible")
		}
		if !hasReason(dec.Reasons, approval.LanguageNotAllowed) {
			t.Errorf("French not rejected in US market: %+v", dec.Reasons)
		}
	})

	t.Run("Remote Canada matches CA market before Remote-anywhere", func(t *testing.T) {
		p := validPosting()
		p.Remote = true
		p.Location = &posting.Location{City: "Montreal", Country: posting.Country("CA")}
		p.Description = "French text"

		dec := engine.Evaluate(p, nil)
		if !dec.Approved() {
			t.Fatalf("remote Canadian French job rejected: %+v", dec.Reasons)
		}
		if dec.Market != "CA" {
			t.Errorf("Market = %q, want CA", dec.Market)
		}
	})

	t.Run("Extensibility future UK-remote rule", func(t *testing.T) {
		policy := approval.DefaultPolicy()
		ukRemote := approval.MarketPolicy{
			Name:         "UK-remote",
			Matches:      approval.RemoteIn(posting.Country("GB")),
			MinAnnualUSD: decimal.RequireFromString("90000"),
			MinHourlyUSD: decimal.RequireFromString("45"),
			Languages:    []posting.Language{posting.English},
		}
		policy.Markets = append([]approval.MarketPolicy{ukRemote}, policy.Markets...)

		customEngine := approval.NewEngine(policy, fakeFX{posting.GBP: decimal.RequireFromString("1.30")}, detector)

		p := validPosting()
		p.Remote = true
		p.Location = &posting.Location{City: "London", Country: posting.Country("GB")}
		p.Salary = &posting.Salary{
			Amount:   decimal.RequireFromString("72000"), // 72,000 * 1.30 = 93,600 USD (>90k, <100k)
			Currency: posting.GBP,
			Unit:     posting.Annual,
		}
		p.Description = "English text"

		decCustom := customEngine.Evaluate(p, nil)
		if !decCustom.Approved() {
			t.Fatalf("expected UK-remote job approved under custom policy: %+v", decCustom.Reasons)
		}
		if decCustom.Market != "UK-remote" {
			t.Errorf("Market = %q, want UK-remote", decCustom.Market)
		}

		defaultEngine := approval.NewEngine(approval.DefaultPolicy(), fakeFX{posting.GBP: decimal.RequireFromString("1.30")}, detector)
		decDefault := defaultEngine.Evaluate(p, nil)
		if decDefault.Approved() {
			t.Fatalf("expected 93.6k USD job rejected under default policy with $100k threshold")
		}
		if !hasReason(decDefault.Reasons, approval.SalaryBelow) {
			t.Errorf("missing SalaryBelow in %+v", decDefault.Reasons)
		}
	})
}

func TestNormalizationIssues(t *testing.T) {
	// Catches: dropping or mismapping incoming normalization issues.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	tests := []struct {
		name       string
		issues     []posting.Issue
		wantReason approval.ReasonCode
		wantPass   bool
	}{
		{
			name: "salary_missing issue blocks approval",
			issues: []posting.Issue{
				{Code: posting.SalaryMissing, Detail: "no salary"},
			},
			wantReason: approval.SalaryMissing,
			wantPass:   false,
		},
		{
			name: "salary_invalid issue blocks approval",
			issues: []posting.Issue{
				{Code: posting.SalaryInvalid, Detail: "negative amount"},
			},
			wantReason: approval.SalaryInvalid,
			wantPass:   false,
		},
		{
			name: "salary_unit_ambiguous issue blocks approval",
			issues: []posting.Issue{
				{Code: posting.SalaryUnitAmbiguous, Detail: "bare 5000"},
			},
			wantReason: approval.SalaryUnitAmbiguous,
			wantPass:   false,
		},
		{
			name: "salary_currency_unknown issue blocks approval",
			issues: []posting.Issue{
				{Code: posting.SalaryCurrencyUnknown, Detail: "no country to infer"},
			},
			wantReason: approval.SalaryCurrencyUnknown,
			wantPass:   false,
		},
		{
			name: "date_invalid does not block approval",
			issues: []posting.Issue{
				{Code: posting.DateInvalid, Detail: "unparseable date"},
			},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPosting()
			if !tc.wantPass {
				p.Salary = nil
			}

			dec := engine.Evaluate(p, tc.issues)
			if tc.wantPass {
				if !dec.Approved() {
					t.Fatalf("job rejected unexpectedly with date_invalid issue: %+v", dec.Reasons)
				}
			} else {
				if dec.Approved() {
					t.Fatalf("job approved unexpectedly with blocking issue: %+v", tc.issues)
				}
				if !hasReason(dec.Reasons, tc.wantReason) {
					t.Errorf("missing expected reason %v in %+v", tc.wantReason, dec.Reasons)
				}
			}
		})
	}
}

func TestPostedOnImmutability(t *testing.T) {
	// Catches: sharing mutable *civil.Date pointer between input Posting and constructed Job.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	date := civil.Date{Year: 2026, Month: 10, Day: 14}
	p := validPosting()
	p.PostedOn = &date

	dec := engine.Evaluate(p, nil)
	if !dec.Approved() {
		t.Fatalf("job rejected unexpectedly: %+v", dec.Reasons)
	}

	// Mutate caller's date
	date.Day = 25

	if dec.Job.PostedOn().Day == 25 {
		t.Errorf("Job.PostedOn() mutated when caller modified source date pointer")
	}
}

func TestNilSalaryDoesNotPanic(t *testing.T) {
	// Catches: nil pointer dereference when evaluating a posting with nil salary and empty issues.
	detector := fakeDetector{
		"Building scalable cloud backend services.": {lang: posting.English, reliable: true},
	}
	engine := approval.NewEngine(approval.DefaultPolicy(), fx.Default, detector)

	p := validPosting()
	p.Salary = nil

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Evaluate() panicked with: %v", r)
		}
	}()

	dec := engine.Evaluate(p, nil)
	if dec.Approved() {
		t.Fatal("Evaluate() approved posting with nil salary")
	}
	if !hasReason(dec.Reasons, approval.SalaryMissing) {
		t.Errorf("missing SalaryMissing reason: %+v", dec.Reasons)
	}
}
