package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams(fee sdk.Coin) Params {
	return Params{
		PastelIdCreateFee: fee,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(sdk.NewCoin("upsl", math.NewInt(10_000_000_000)))
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}

// Validate validates the set of params
func (p Params) Validate() error {
	return validatePastelIDCreateFee(p.PastelIdCreateFee)
}

func validatePastelIDCreateFee(fee sdk.Coin) error {
	if !fee.IsValid() {
		return fmt.Errorf("invalid pastelID create fee: %s", fee)
	}
	if fee.IsZero() {
		return fmt.Errorf("pastelID create fee cannot be zero")
	}
	return nil
}
