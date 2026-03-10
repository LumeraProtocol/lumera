package types

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		PaymentPeriodBlocks:         100800, // ~7 days at 6s blocks
		ValidatorRewardShareBps:     100,    // 1%
		RegistrationFeeShareBps:     200,    // 2%
		MinCascadeBytesForPayment:   1073741824, // 1 GB
		NewSnRampUpPeriods:          4,
		MeasurementSmoothingPeriods: 4,
		UsageGrowthCapBpsPerPeriod:  1000, // 10% max growth per period
	}
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.PaymentPeriodBlocks == 0 {
		return ErrInvalidParams.Wrap("payment_period_blocks must be > 0")
	}
	if p.ValidatorRewardShareBps > 10000 {
		return ErrInvalidParams.Wrap("validator_reward_share_bps must be <= 10000")
	}
	if p.RegistrationFeeShareBps > 10000 {
		return ErrInvalidParams.Wrap("registration_fee_share_bps must be <= 10000")
	}
	return nil
}
