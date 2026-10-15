package approval

import (
	"fmt"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type Engine struct {
	policy Policy
	fx     FXRates
	lang   LanguageDetector
}

func NewEngine(p Policy, fx FXRates, lang LanguageDetector) *Engine {
	return &Engine{
		policy: p,
		fx:     fx,
		lang:   lang,
	}
}

func (e *Engine) Evaluate(p posting.Posting, issues []posting.Issue) Decision {
	var reasons []Reason
	hasSalaryIssue := false

	for _, issue := range issues {
		switch issue.Code {
		case posting.SalaryMissing:
			reasons = append(reasons, Reason{Code: SalaryMissing, Detail: issue.Detail})
			hasSalaryIssue = true
		case posting.SalaryInvalid:
			reasons = append(reasons, Reason{Code: SalaryInvalid, Detail: issue.Detail})
			hasSalaryIssue = true
		case posting.SalaryUnitAmbiguous:
			reasons = append(reasons, Reason{Code: SalaryUnitAmbiguous, Detail: issue.Detail})
			hasSalaryIssue = true
		case posting.SalaryCurrencyUnknown:
			reasons = append(reasons, Reason{Code: SalaryCurrencyUnknown, Detail: issue.Detail})
			hasSalaryIssue = true
		case posting.DateInvalid:
			// date_invalid does not block approval
		}
	}

	if p.Salary == nil && !hasSalaryIssue {
		reasons = append(reasons, Reason{Code: SalaryMissing, Detail: "missing salary"})
	}

	if strings.TrimSpace(p.Title) == "" {
		reasons = append(reasons, Reason{Code: TitleMissing, Detail: "missing title"})
	}

	if p.EmploymentType != posting.FullTime {
		reasons = append(reasons, Reason{
			Code:   NotFullTime,
			Detail: fmt.Sprintf("employment type %s is not full_time", p.EmploymentType),
		})
	}

	if e.policy.StaffingTypes != nil && e.policy.StaffingTypes[p.CompanyType] {
		reasons = append(reasons, Reason{
			Code:   StaffingFirm,
			Detail: fmt.Sprintf("company type %s is excluded", p.CompanyType),
		})
	}

	market := e.policy.Fallback
	marketMatched := false
	for _, m := range e.policy.Markets {
		if m.Matches != nil && m.Matches(p) {
			market = m
			marketMatched = true
			break
		}
	}

	if !marketMatched {
		reasons = append(reasons, Reason{
			Code:   LocationNotEligible,
			Detail: "location does not match any eligible market",
		})
	}

	var fxApplied *FXApplied
	var annualUSD decimal.Decimal
	if !hasSalaryIssue && p.Salary != nil && e.fx != nil {
		usd, rate, err := e.fx.ToUSD(p.Salary.Amount, p.Salary.Currency)
		if err != nil {
			reasons = append(reasons, Reason{
				Code:   SalaryCurrencyUnknown,
				Detail: err.Error(),
			})
		} else {
			fxApplied = &FXApplied{
				Rate:      rate,
				AmountUSD: usd,
				Inferred:  p.Salary.CurrencyGuess,
			}

			if p.Salary.Unit == posting.Hourly {
				if !usd.GreaterThan(market.MinHourlyUSD) {
					reasons = append(reasons, Reason{
						Code:   SalaryBelow,
						Detail: fmt.Sprintf("%s USD/h <= %s USD/h", usd.StringFixed(2), market.MinHourlyUSD.StringFixed(2)),
					})
				}
				annualUSD = usd.Mul(decimal.NewFromInt(2080))
			} else {
				if !usd.GreaterThan(market.MinAnnualUSD) {
					reasons = append(reasons, Reason{
						Code:   SalaryBelow,
						Detail: fmt.Sprintf("%s USD <= %s USD", usd.StringFixed(2), market.MinAnnualUSD.StringFixed(2)),
					})
				}
				annualUSD = usd
			}
			fxApplied.AnnualUSD = annualUSD
		}
	}

	var detectedLang posting.Language
	reliable := false
	if e.lang != nil {
		detectedLang, reliable = e.lang.Detect(p.Description)
	}
	if !reliable {
		detectedLang = p.DeclaredLanguage
	}

	if detectedLang == "" {
		reasons = append(reasons, Reason{
			Code:   LanguageUndetermined,
			Detail: "language could not be reliably determined",
		})
	} else {
		allowed := false
		for _, l := range market.Languages {
			if l == detectedLang {
				allowed = true
				break
			}
		}
		if !allowed {
			reasons = append(reasons, Reason{
				Code:   LanguageNotAllowed,
				Detail: fmt.Sprintf("language %s not allowed in market %s", detectedLang, market.Name),
			})
		}
	}

	var job *Job
	if len(reasons) == 0 && p.Salary != nil {
		var city string
		var countryPtr *posting.Country
		if p.Location != nil {
			city = p.Location.City
			if p.Location.Country != "" {
				c := p.Location.Country
				countryPtr = &c
			}
		}

		var postedOnPtr *civil.Date
		if p.PostedOn != nil {
			d := *p.PostedOn
			postedOnPtr = &d
		}

		job = &Job{
			title:     p.Title,
			company:   p.Company,
			city:      city,
			country:   countryPtr,
			remote:    p.Remote,
			salary:    *p.Salary,
			annualUSD: annualUSD,
			lang:      detectedLang,
			postedOn:  postedOnPtr,
		}
	}

	return Decision{
		Job:           job,
		Reasons:       reasons,
		Market:        market.Name,
		FX:            fxApplied,
		PolicyVersion: e.policy.Version,
	}
}
