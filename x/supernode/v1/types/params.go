package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

var (
	KeyMinimumStakeForSn = []byte("MinimumStakeForSn")
	// TODO: Determine the default value
	DefaultMinimumStakeForSn = sdk.NewInt64Coin("ulume", 0)
)

var (
	KeyReportingThreshold = []byte("ReportingThreshold")
	// TODO: Determine the default value
	DefaultReportingThreshold uint64 = 0
)

var (
	KeySlashingThreshold = []byte("SlashingThreshold")
	// TODO: Determine the default value
	DefaultSlashingThreshold uint64 = 0
)

var (
	KeyMetricsThresholds = []byte("MetricsThresholds")
	// TODO: Determine the default value
	DefaultMetricsThresholds string = ""
)

var (
	KeyEvidenceRetentionPeriod = []byte("EvidenceRetentionPeriod")
	// TODO: Determine the default value
	DefaultEvidenceRetentionPeriod string = ""
)

var (
	KeySlashingFraction = []byte("SlashingFraction")
	// TODO: Determine the default value
	DefaultSlashingFraction string = ""
)

var (
	KeyInactivityPenaltyPeriod = []byte("InactivityPenaltyPeriod")
	// TODO: Determine the default value
	DefaultInactivityPenaltyPeriod string = ""
)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams(
	minimumStakeForSn sdk.Coin,
	reportingThreshold uint64,
	slashingThreshold uint64,
	metricsThresholds string,
	evidenceRetentionPeriod string,
	slashingFraction string,
	inactivityPenaltyPeriod string,
) Params {
	return Params{
		MinimumStakeForSn:       minimumStakeForSn,
		ReportingThreshold:      reportingThreshold,
		SlashingThreshold:       slashingThreshold,
		MetricsThresholds:       metricsThresholds,
		EvidenceRetentionPeriod: evidenceRetentionPeriod,
		SlashingFraction:        slashingFraction,
		InactivityPenaltyPeriod: inactivityPenaltyPeriod,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(
		DefaultMinimumStakeForSn,
		DefaultReportingThreshold,
		DefaultSlashingThreshold,
		DefaultMetricsThresholds,
		DefaultEvidenceRetentionPeriod,
		DefaultSlashingFraction,
		DefaultInactivityPenaltyPeriod,
	)
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyMinimumStakeForSn, &p.MinimumStakeForSn, validateMinimumStakeForSn),
		paramtypes.NewParamSetPair(KeyReportingThreshold, &p.ReportingThreshold, validateReportingThreshold),
		paramtypes.NewParamSetPair(KeySlashingThreshold, &p.SlashingThreshold, validateSlashingThreshold),
		paramtypes.NewParamSetPair(KeyMetricsThresholds, &p.MetricsThresholds, validateMetricsThresholds),
		paramtypes.NewParamSetPair(KeyEvidenceRetentionPeriod, &p.EvidenceRetentionPeriod, validateEvidenceRetentionPeriod),
		paramtypes.NewParamSetPair(KeySlashingFraction, &p.SlashingFraction, validateSlashingFraction),
		paramtypes.NewParamSetPair(KeyInactivityPenaltyPeriod, &p.InactivityPenaltyPeriod, validateInactivityPenaltyPeriod),
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMinimumStakeForSn(p.MinimumStakeForSn); err != nil {
		return err
	}

	if err := validateReportingThreshold(p.ReportingThreshold); err != nil {
		return err
	}

	if err := validateSlashingThreshold(p.SlashingThreshold); err != nil {
		return err
	}

	if err := validateMetricsThresholds(p.MetricsThresholds); err != nil {
		return err
	}

	if err := validateEvidenceRetentionPeriod(p.EvidenceRetentionPeriod); err != nil {
		return err
	}

	if err := validateSlashingFraction(p.SlashingFraction); err != nil {
		return err
	}

	if err := validateInactivityPenaltyPeriod(p.InactivityPenaltyPeriod); err != nil {
		return err
	}

	return nil
}

// validateMinimumStakeForSn validates the MinimumStakeForSn param
func validateMinimumStakeForSn(v interface{}) error {
	minimumStakeForSn, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = minimumStakeForSn

	return nil
}

// validateReportingThreshold validates the ReportingThreshold param
func validateReportingThreshold(v interface{}) error {
	reportingThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = reportingThreshold

	return nil
}

// validateSlashingThreshold validates the SlashingThreshold param
func validateSlashingThreshold(v interface{}) error {
	slashingThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = slashingThreshold

	return nil
}

// validateMetricsThresholds validates the MetricsThresholds param
func validateMetricsThresholds(v interface{}) error {
	metricsThresholds, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = metricsThresholds

	return nil
}

// validateEvidenceRetentionPeriod validates the EvidenceRetentionPeriod param
func validateEvidenceRetentionPeriod(v interface{}) error {
	evidenceRetentionPeriod, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = evidenceRetentionPeriod

	return nil
}

// validateSlashingFraction validates the SlashingFraction param
func validateSlashingFraction(v interface{}) error {
	slashingFraction, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = slashingFraction

	return nil
}

// validateInactivityPenaltyPeriod validates the InactivityPenaltyPeriod param
func validateInactivityPenaltyPeriod(v interface{}) error {
	inactivityPenaltyPeriod, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	// TODO implement validation
	_ = inactivityPenaltyPeriod

	return nil
}
