package domain_test

import (
	"encoding/json"
	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMoneyPrecision(t *testing.T) {
	for _, value := range []string{"-1", "0.001", "10000000000000000", "not-money", "1e-1000000000"} {
		t.Run(value, func(t *testing.T) { _, err := domain.NewMoney(value, "RUB"); require.Error(t, err) })
	}
	price, err := domain.NewMoney("0.10", "RUB")
	require.NoError(t, err)
	total := price.Amount.Mul(decimal.NewFromInt(3))
	require.Equal(t, "0.3", total.String())
	data, err := json.Marshal(price)
	require.NoError(t, err)
	require.JSONEq(t, `{"amount":"0.1","currency":"RUB"}`, string(data))
	_, err = domain.NewMoney("1.00", "USD")
	require.Error(t, err)
}
