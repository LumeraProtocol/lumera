package common

import (
	"fmt"
	"strconv"
	"strings"
)

// ChainDenom is the Lumera native token denom used for funding and activity.
const ChainDenom = "ulume"

// Coin is a parsed amount/denom pair (e.g. 10000000ulume).
type Coin struct {
	Amount int64
	Denom  string
}

// String renders the coin in Cosmos "<amount><denom>" form.
func (c Coin) String() string {
	return strconv.FormatInt(c.Amount, 10) + c.Denom
}

// ParseCoin parses a Cosmos coin string such as "10000000ulume" into a Coin.
// The amount must be a non-negative integer and the denom must be non-empty.
func ParseCoin(s string) (Coin, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Coin{}, fmt.Errorf("empty coin string")
	}
	// Split at the first non-digit/non-sign character: the amount prefix.
	i := 0
	for i < len(trimmed) && (trimmed[i] == '-' || trimmed[i] == '+' || (trimmed[i] >= '0' && trimmed[i] <= '9')) {
		i++
	}
	amountStr, denom := trimmed[:i], trimmed[i:]
	if amountStr == "" || amountStr == "-" || amountStr == "+" {
		return Coin{}, fmt.Errorf("coin %q has no amount", s)
	}
	if denom == "" {
		return Coin{}, fmt.Errorf("coin %q has no denom", s)
	}
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return Coin{}, fmt.Errorf("coin %q has invalid amount: %w", s, err)
	}
	if amount < 0 {
		return Coin{}, fmt.Errorf("coin %q has negative amount", s)
	}
	return Coin{Amount: amount, Denom: denom}, nil
}

// ValidateMaxAccountAmount parses and validates the -max-account-amount flag:
// it must be a positive amount denominated in the chain denom.
func ValidateMaxAccountAmount(s string) (Coin, error) {
	c, err := ParseCoin(s)
	if err != nil {
		return Coin{}, err
	}
	if c.Amount <= 0 {
		return Coin{}, fmt.Errorf("max-account-amount must be positive, got %q", s)
	}
	if c.Denom != ChainDenom {
		return Coin{}, fmt.Errorf("max-account-amount must be denominated in %s, got %q", ChainDenom, c.Denom)
	}
	return c, nil
}
