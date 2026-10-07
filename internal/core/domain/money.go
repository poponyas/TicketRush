package domain

import (
	"fmt"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/shopspring/decimal"
)

const CurrencyRUB = "RUB"

// Amount is in rubles, not kopecks. SQL representation: NUMERIC(18,2).
// Parse strings with decimal.NewFromString; do not go through float64.
type Money struct {
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
}

func NewMoney(amount string, currency string) (Money, error) {
	if len(amount) > 64 {
		return Money{}, fmt.Errorf("%w: amount representation is too long", coreerrors.ErrInvalidArgument)
	}
	value, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("%w: invalid amount", coreerrors.ErrInvalidArgument)
	}
	money := Money{Amount: value, Currency: currency}
	if err := money.Validate(); err != nil {
		return Money{}, err
	}
	return money, nil
}
func (m Money) Validate() error {
	if m.Currency != CurrencyRUB {
		return fmt.Errorf("%w: only RUB is supported", coreerrors.ErrInvalidArgument)
	}
	// Bound the exponent before rounding or comparing arbitrary input.
	if m.Amount.Exponent() < -18 || m.Amount.Exponent() > 16 {
		return fmt.Errorf("%w: amount exponent is out of range", coreerrors.ErrInvalidArgument)
	}
	if m.Amount.IsNegative() || !m.Amount.Equal(m.Amount.Round(2)) || m.Amount.GreaterThanOrEqual(decimal.New(1, 16)) {
		return fmt.Errorf("%w: amount must be nonnegative, fit NUMERIC(18,2) and have at most two fractional digits", coreerrors.ErrInvalidArgument)
	}
	return nil
}
