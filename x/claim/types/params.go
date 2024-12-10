package types

import (
	"time"
)

func NewParams(enableClaims bool, claimEntTime int64, maxClaimsPerBlock uint64) Params {
	return Params{
		EnableClaims:      enableClaims,
		ClaimEndTime:      claimEntTime,
		MaxClaimsPerBlock: maxClaimsPerBlock,
	}
}

func DefaultParams() Params {
	return NewParams(
		true,
		time.Now().Add(time.Hour*400).Unix(),
		100,
	)
}

func (p Params) Validate() error {
	if p.MaxClaimsPerBlock < 1 {
		return ErrInvalidParamMaxClaims
	}
	if p.ClaimEndTime < 1 {
		return ErrInvalidParamClaimDuration
	}

	return nil
}
