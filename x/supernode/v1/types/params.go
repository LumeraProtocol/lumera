package types

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
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

var (
	KeyMetricsUpdateInterval     = []byte("MetricsUpdateInterval")
	KeyMetricsGracePeriodBlocks  = []byte("MetricsGracePeriodBlocks")
	KeyMetricsFreshnessMaxBlocks = []byte("MetricsFreshnessMaxBlocks")
	KeyMinSupernodeVersion       = []byte("MinSupernodeVersion")
	KeyMinCPUCores               = []byte("MinCPUCores")
	KeyMaxCPUUsagePercent        = []byte("MaxCPUUsagePercent")
	KeyMinMemGB                  = []byte("MinMemGB")
	KeyMaxMemUsagePercent        = []byte("MaxMemUsagePercent")
	KeyMinStorageGB              = []byte("MinStorageGB")
	KeyMaxStorageUsagePercent    = []byte("MaxStorageUsagePercent")
	KeyRequiredOpenPorts         = []byte("RequiredOpenPorts")
)

const (
	DefaultMetricsUpdateInterval     uint64  = 1000
	DefaultMetricsGracePeriodBlocks  uint64  = 100
	DefaultMetricsFreshnessMaxBlocks uint64  = 5000
	DefaultMinSupernodeVersion               = "2.0.0"
	DefaultMinCPUCores               float64 = 8
	DefaultMaxCPUUsagePercent        float64 = 90.0
	DefaultMinMemGB                  float64 = 16
	DefaultMaxMemUsagePercent        float64 = 90.0
	DefaultMinStorageGB              float64 = 1000
	DefaultMaxStorageUsagePercent    float64 = 90.0
)

var DefaultRequiredOpenPorts = []uint32{4444, 4445, 8002}

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
	metricsUpdateInterval uint64,
	metricsGracePeriodBlocks uint64,
	metricsFreshnessMaxBlocks uint64,
	minSupernodeVersion string,
	minCPUCores float64,
	maxCPUUsagePercent float64,
	minMemGB float64,
	maxMemUsagePercent float64,
	minStorageGB float64,
	maxStorageUsagePercent float64,
	requiredOpenPorts []uint32,
) Params {
	return Params{
		MinimumStakeForSn:         minimumStakeForSn,
		ReportingThreshold:        reportingThreshold,
		SlashingThreshold:         slashingThreshold,
		MetricsThresholds:         metricsThresholds,
		EvidenceRetentionPeriod:   evidenceRetentionPeriod,
		SlashingFraction:          slashingFraction,
		InactivityPenaltyPeriod:   inactivityPenaltyPeriod,
		MetricsUpdateInterval:     metricsUpdateInterval,
		MetricsGracePeriodBlocks:  metricsGracePeriodBlocks,
		MetricsFreshnessMaxBlocks: metricsFreshnessMaxBlocks,
		MinSupernodeVersion:       minSupernodeVersion,
		MinCpuCores:               minCPUCores,
		MaxCpuUsagePercent:        maxCPUUsagePercent,
		MinMemGb:                  minMemGB,
		MaxMemUsagePercent:        maxMemUsagePercent,
		MinStorageGb:              minStorageGB,
		MaxStorageUsagePercent:    maxStorageUsagePercent,
		RequiredOpenPorts:         requiredOpenPorts,
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
		DefaultMetricsUpdateInterval,
		DefaultMetricsGracePeriodBlocks,
		DefaultMetricsFreshnessMaxBlocks,
		DefaultMinSupernodeVersion,
		DefaultMinCPUCores,
		DefaultMaxCPUUsagePercent,
		DefaultMinMemGB,
		DefaultMaxMemUsagePercent,
		DefaultMinStorageGB,
		DefaultMaxStorageUsagePercent,
		DefaultRequiredOpenPorts,
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
		paramtypes.NewParamSetPair(KeyMetricsUpdateInterval, &p.MetricsUpdateInterval, validatePositiveUint64("metrics update interval")),
		paramtypes.NewParamSetPair(KeyMetricsGracePeriodBlocks, &p.MetricsGracePeriodBlocks, validatePositiveUint64("metrics grace period")),
		paramtypes.NewParamSetPair(KeyMetricsFreshnessMaxBlocks, &p.MetricsFreshnessMaxBlocks, validatePositiveUint64("metrics freshness max blocks")),
		paramtypes.NewParamSetPair(KeyMinSupernodeVersion, &p.MinSupernodeVersion, validateVersionString),
		paramtypes.NewParamSetPair(KeyMinCPUCores, &p.MinCpuCores, validateNonNegative("min cpu cores")),
		paramtypes.NewParamSetPair(KeyMaxCPUUsagePercent, &p.MaxCpuUsagePercent, validatePercent("max cpu usage percent")),
		paramtypes.NewParamSetPair(KeyMinMemGB, &p.MinMemGb, validateNonNegative("min mem gb")),
		paramtypes.NewParamSetPair(KeyMaxMemUsagePercent, &p.MaxMemUsagePercent, validatePercent("max mem usage percent")),
		paramtypes.NewParamSetPair(KeyMinStorageGB, &p.MinStorageGb, validateNonNegative("min storage gb")),
		paramtypes.NewParamSetPair(KeyMaxStorageUsagePercent, &p.MaxStorageUsagePercent, validatePercent("max storage usage percent")),
		paramtypes.NewParamSetPair(KeyRequiredOpenPorts, &p.RequiredOpenPorts, validateRequiredPorts),
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

	if err := validatePositiveUint64("metrics update interval")(p.MetricsUpdateInterval); err != nil {
		return err
	}
	if err := validatePositiveUint64("metrics grace period")(p.MetricsGracePeriodBlocks); err != nil {
		return err
	}
	if err := validatePositiveUint64("metrics freshness max blocks")(p.MetricsFreshnessMaxBlocks); err != nil {
		return err
	}
	if err := validateVersionString(p.MinSupernodeVersion); err != nil {
		return err
	}
	if err := validateNonNegative("min cpu cores")(p.MinCpuCores); err != nil {
		return err
	}
	if err := validatePercent("max cpu usage percent")(p.MaxCpuUsagePercent); err != nil {
		return err
	}
	if err := validateNonNegative("min mem gb")(p.MinMemGb); err != nil {
		return err
	}
	if err := validatePercent("max mem usage percent")(p.MaxMemUsagePercent); err != nil {
		return err
	}
	if err := validateNonNegative("min storage gb")(p.MinStorageGb); err != nil {
		return err
	}
	if err := validatePercent("max storage usage percent")(p.MaxStorageUsagePercent); err != nil {
		return err
	}
	if err := validateRequiredPorts(p.RequiredOpenPorts); err != nil {
		return err
	}

	return nil
}

// validateMinimumStakeForSn validates the MinimumStakeForSn param
func validateMinimumStakeForSn(v interface{}) error {
	coin, ok := v.(sdk.Coin)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	// Perform validation on the coin
	return coin.Validate()
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

func validatePositiveUint64(field string) func(interface{}) error {
	return func(v interface{}) error {
		value, ok := v.(uint64)
		if !ok {
			return fmt.Errorf("invalid parameter type for %s: %T", field, v)
		}
		if value == 0 {
			return fmt.Errorf("%s must be greater than zero", field)
		}
		return nil
	}
}

func validateNonNegative(field string) func(interface{}) error {
	return func(v interface{}) error {
		value, ok := v.(float64)
		if !ok {
			return fmt.Errorf("invalid parameter type for %s: %T", field, v)
		}
		if value < 0 {
			return fmt.Errorf("%s must be non-negative", field)
		}
		return nil
	}
}

func validatePercent(field string) func(interface{}) error {
	return func(v interface{}) error {
		value, ok := v.(float64)
		if !ok {
			return fmt.Errorf("invalid parameter type for %s: %T", field, v)
		}
		if value < 0 || value > 100 {
			return fmt.Errorf("%s must be between 0 and 100", field)
		}
		return nil
	}
}

func validateVersionString(v interface{}) error {
	version, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}
	if version == "" {
		return fmt.Errorf("min supernode version cannot be empty")
	}
	if _, err := semver.NewVersion(version); err != nil {
		return fmt.Errorf("invalid semantic version: %w", err)
	}
	return nil
}

func validateRequiredPorts(v interface{}) error {
	ports, ok := v.([]uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type for required ports: %T", v)
	}
	for _, port := range ports {
		if port == 0 {
			return fmt.Errorf("required port value must be non-zero")
		}
	}
	return nil
}
