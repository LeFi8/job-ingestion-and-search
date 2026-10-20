package fx

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

var ErrUnknownCurrency = errors.New("fx: unknown currency")

type Static map[posting.Currency]decimal.Decimal

var Default = Static{
	posting.USD: decimal.RequireFromString("1"),
	posting.CAD: decimal.RequireFromString("0.70"),
	posting.GBP: decimal.RequireFromString("1.32"),
	posting.EUR: decimal.RequireFromString("1.11"),
	posting.PLN: decimal.RequireFromString("0.25"),
}

func (s Static) ToUSD(amount decimal.Decimal, cur posting.Currency) (decimal.Decimal, decimal.Decimal, error) {
	rate, ok := s[cur]
	if !ok {
		return decimal.Zero, decimal.Zero, fmt.Errorf("%w: %s", ErrUnknownCurrency, cur)
	}
	return amount.Mul(rate), rate, nil
}
