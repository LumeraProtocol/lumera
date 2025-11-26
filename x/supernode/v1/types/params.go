package types

import (
	"fmt"

	semver "github.com/Masterminds/semver/v3"
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
	KeyMetricsTiming            = []byte("MetricsTiming")
	DefaultMetricsTiming uint64 = 300
)

var (
	KeyMetricsVersion            = []byte("MetricsVersion")
	DefaultMetricsVersion string = "1.0.0"
)

var (
	KeyCPUThreshold            = []byte("CPUThreshold")
	DefaultCPUThreshold uint64 = 80
)

var (
	KeyMemoryThreshold            = []byte("MemoryThreshold")
	DefaultMemoryThreshold uint64 = 80
)

var (
	KeyStorageThreshold            = []byte("StorageThreshold")
	DefaultStorageThreshold uint64 = 80
)

var (
	KeyRequiredPorts     = []byte("RequiredPorts")
	DefaultRequiredPorts = []uint32{22, 26656, 26657, 9090, 1317}
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
	metricsTiming uint64,
	metricsVersion string,
	cpuThreshold uint64,
	memoryThreshold uint64,
	storageThreshold uint64,
	requiredPorts []uint32,
) Params {
	return Params{
		MinimumStakeForSn:       minimumStakeForSn,
		ReportingThreshold:      reportingThreshold,
		SlashingThreshold:       slashingThreshold,
		MetricsThresholds:       metricsThresholds,
		EvidenceRetentionPeriod: evidenceRetentionPeriod,
		SlashingFraction:        slashingFraction,
		InactivityPenaltyPeriod: inactivityPenaltyPeriod,
		MetricsTiming:           metricsTiming,
		MetricsVersion:          metricsVersion,
		CPUThreshold:            cpuThreshold,
		MemoryThreshold:         memoryThreshold,
		StorageThreshold:        storageThreshold,
		RequiredPorts:           requiredPorts,
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
		DefaultMetricsTiming,
		DefaultMetricsVersion,
		DefaultCPUThreshold,
		DefaultMemoryThreshold,
		DefaultStorageThreshold,
		DefaultRequiredPorts,
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
		paramtypes.NewParamSetPair(KeyMetricsTiming, &p.MetricsTiming, validateMetricsTiming),
		paramtypes.NewParamSetPair(KeyMetricsVersion, &p.MetricsVersion, validateMetricsVersion),
		paramtypes.NewParamSetPair(KeyCPUThreshold, &p.CPUThreshold, validateCPUThreshold),
		paramtypes.NewParamSetPair(KeyMemoryThreshold, &p.MemoryThreshold, validateMemoryThreshold),
		paramtypes.NewParamSetPair(KeyStorageThreshold, &p.StorageThreshold, validateStorageThreshold),
		paramtypes.NewParamSetPair(KeyRequiredPorts, &p.RequiredPorts, validateRequiredPorts),
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

	if err := validateMetricsTiming(p.MetricsTiming); err != nil {
		return err
	}

	if err := validateMetricsVersion(p.MetricsVersion); err != nil {
		return err
	}

	if err := validateCPUThreshold(p.CPUThreshold); err != nil {
		return err
	}

	if err := validateMemoryThreshold(p.MemoryThreshold); err != nil {
		return err
	}

	if err := validateStorageThreshold(p.StorageThreshold); err != nil {
		return err
	}

	if err := validateRequiredPorts(p.RequiredPorts); err != nil {
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

	if reportingThreshold == 0 {
		return fmt.Errorf("reporting threshold must be positive")
	}

	return nil
}

// validateSlashingThreshold validates the SlashingThreshold param
func validateSlashingThreshold(v interface{}) error {
	slashingThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if slashingThreshold == 0 {
		return fmt.Errorf("slashing threshold must be positive")
	}

	return nil
}

// validateMetricsThresholds validates the MetricsThresholds param
func validateMetricsThresholds(v interface{}) error {
	metricsThresholds, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if metricsThresholds == "" {
		return fmt.Errorf("metrics thresholds cannot be empty")
	}

	return nil
}

// validateEvidenceRetentionPeriod validates the EvidenceRetentionPeriod param
func validateEvidenceRetentionPeriod(v interface{}) error {
	evidenceRetentionPeriod, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if evidenceRetentionPeriod == "" {
		return fmt.Errorf("evidence retention period cannot be empty")
	}

	return nil
}

// validateSlashingFraction validates the SlashingFraction param
func validateSlashingFraction(v interface{}) error {
	slashingFraction, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if slashingFraction == "" {
		return fmt.Errorf("slashing fraction cannot be empty")
	}

	return nil
}

// validateInactivityPenaltyPeriod validates the InactivityPenaltyPeriod param
func validateInactivityPenaltyPeriod(v interface{}) error {
	inactivityPenaltyPeriod, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if inactivityPenaltyPeriod == "" {
		return fmt.Errorf("inactivity penalty period cannot be empty")
	}

	return nil
}

func validateMetricsTiming(v interface{}) error {
	metricsTiming, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if metricsTiming == 0 {
		return fmt.Errorf("metrics timing must be positive")
	}

	return nil
}

func validateMetricsVersion(v interface{}) error {
	metricsVersion, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if metricsVersion == "" {
		return fmt.Errorf("metrics version cannot be empty")
	}

	if _, err := semver.NewVersion(metricsVersion); err != nil {
		return fmt.Errorf("invalid metrics version: %w", err)
	}

	return nil
}

func validateCPUThreshold(v interface{}) error {
	cpuThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if cpuThreshold > 100 {
		return fmt.Errorf("cpu threshold must be between 0 and 100")
	}

	return nil
}

func validateMemoryThreshold(v interface{}) error {
	memoryThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if memoryThreshold > 100 {
		return fmt.Errorf("memory threshold must be between 0 and 100")
	}

	return nil
}

func validateStorageThreshold(v interface{}) error {
	storageThreshold, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if storageThreshold > 100 {
		return fmt.Errorf("storage threshold must be between 0 and 100")
	}

	return nil
}

func validateRequiredPorts(v interface{}) error {
	requiredPorts, ok := v.([]uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	seen := make(map[uint32]struct{})
	for _, port := range requiredPorts {
		if port == 0 || port > 65535 {
			return fmt.Errorf("required ports must be within valid TCP/UDP range")
		}
		if _, exists := seen[port]; exists {
			return fmt.Errorf("duplicate port %d in required ports", port)
		}
		seen[port] = struct{}{}
	}

	return nil
}
