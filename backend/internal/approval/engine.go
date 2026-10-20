package approval

import (
	"errors"
	"slices"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

const (
	hoursPerYear = 2080
	// storedDecimals matches salary_usd_annual numeric(14,2), so the decision
	// compares the value that is stored.
	storedDecimals = 2
)

type Engine struct {
	policy Policy
	fx     FXRates
	lang   LanguageDetector
}

func NewEngine(p Policy, fx FXRates, lang LanguageDetector) (*Engine, error) {
	switch {
	case fx == nil:
		return nil, errors.New("approval: engine needs FX rates")
	case lang == nil:
		return nil, errors.New("approval: engine needs a language detector")
	case p.Version == "":
		return nil, errors.New("approval: policy has no version")
	case len(p.Rules) == 0:
		return nil, errors.New("approval: policy has no rules")
	}
	return &Engine{
		policy: p,
		fx:     fx,
		lang:   lang,
	}, nil
}

func (e *Engine) Evaluate(p posting.Posting, issues []posting.Issue) Decision {
	f := e.facts(p, issues)
	var reasons []Reason
	for _, rule := range e.policy.Rules {
		reasons = append(reasons, rule(f)...)
	}
	return Decision{
		approved:      len(reasons) == 0 && p.Salary != nil,
		Reasons:       reasons,
		Market:        f.Market.Name,
		FX:            f.FX,
		Language:      f.Language,
		PolicyVersion: e.policy.Version,
	}
}

func (e *Engine) facts(p posting.Posting, issues []posting.Issue) Facts {
	f := Facts{Posting: p, Issues: issues}
	f.Market, f.Matched = e.market(p)

	blocked := slices.ContainsFunc(issues, func(i posting.Issue) bool {
		_, ok := blockingIssueReasons[i.Code]
		return ok
	})
	if !blocked && p.Salary != nil {
		usd, rate, err := e.fx.ToUSD(p.Salary.Amount, p.Salary.Currency)
		if err != nil {
			f.FXErr = err
		} else {
			usd = usd.Round(storedDecimals)
			annual := usd
			if p.Salary.Unit == posting.Hourly {
				annual = usd.Mul(decimal.NewFromInt(hoursPerYear))
			}
			f.USD = usd
			f.FX = &FXApplied{Rate: rate, AnnualUSD: annual, Inferred: p.Salary.CurrencyGuess}
		}
	}

	lang, reliable := e.lang.Detect(p.Description)
	if !reliable {
		lang = p.DeclaredLanguage
	}
	f.Language = lang
	return f
}

func (e *Engine) market(p posting.Posting) (MarketPolicy, bool) {
	for _, m := range e.policy.Markets {
		if m.Matches != nil && m.Matches(p) {
			return m, true
		}
	}
	if p.Remote && e.policy.RemoteAnywhere != nil {
		return *e.policy.RemoteAnywhere, true
	}
	return e.policy.Fallback, false
}
