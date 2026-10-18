package posting

import (
	"fmt"

	"github.com/shopspring/decimal"
)

func SalaryDisplay(amount *decimal.Decimal, currency string, unit PayUnit) (string, error) {
	if amount == nil {
		return "", nil
	}
	var per string
	switch unit {
	case Hourly:
		per = "hour"
	case Annual:
		per = "year"
	default:
		return "", fmt.Errorf("unsupported salary unit %q", unit)
	}
	return amount.StringFixed(2) + " " + currency + " / " + per, nil
}
