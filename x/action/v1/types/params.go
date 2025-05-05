package types

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Parameter keys
var (
	KeyBaseActionFee        = []byte("BaseActionFee")
	KeyFeePerByte           = []byte("FeePerByte")
	KeyMaxActionsPerBlock   = []byte("MaxActionsPerBlock")
	KeyMinSuperNodes        = []byte("MinSuperNodes")
	KeyMaxDdAndFingerprints = []byte("MaxDdAndFingerprints")
	KeyMaxRaptorQSymbols    = []byte("MaxRaptorQSymbols")
	KeyExpirationDuration   = []byte("ExpirationDuration")
	KeyMinProcessingTime    = []byte("MinProcessingTime")
	KeyMaxProcessingTime    = []byte("MaxProcessingTime")
	KeySuperNodeFeeShare    = []byte("SuperNodeFeeShare")
	KeyFoundationFeeShare   = []byte("FoundationFeeShare")
)

// Default parameter values
var (
	DefaultBaseActionFee        = sdk.NewCoin("ulume", math.NewInt(10000)) // 0.01 LUME
	DefaultFeePerByte           = sdk.NewCoin("ulume", math.NewInt(100))   // 0.0001 LUME per byte
	DefaultMaxActionsPerBlock   = uint64(10)                               // 100 actions per block
	DefaultMinSuperNodes        = uint64(3)                                // Minimum 3 super nodes
	DefaultMaxDdAndFingerprints = uint64(50)                               // Maximum 1000 DDs and fingerprints
	DefaultMaxRaptorQSymbols    = uint64(50)                               // Maximum 10000 RaptorQ symbols
	DefaultExpirationDuration   = 24 * time.Hour                           // 24 hour expiration
	DefaultMinProcessingTime    = 1 * time.Minute                          // 1 minute minimum processing time
	DefaultMaxProcessingTime    = 1 * time.Hour                            // 1 hour maximum processing time
	DefaultSuperNodeFeeShare    = "1.000000000000000000"                   // 1.0 (100%)
	DefaultFoundationFeeShare   = "0.000000000000000000"                   // 0.0 (0%)
)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams(
	baseActionFee sdk.Coin,
	feePerByte sdk.Coin,
	maxActionsPerBlock uint64,
	minSuperNodes uint64,
	maxDdAndFingerprints uint64,
	maxRaptorQSymbols uint64,
	expirationDuration time.Duration,
	minProcessingTime time.Duration,
	maxProcessingTime time.Duration,
	superNodeFeeShare string,
	foundationFeeShare string,
) Params {
	return Params{
		BaseActionFee:        baseActionFee,
		FeePerByte:           feePerByte,
		MaxActionsPerBlock:   maxActionsPerBlock,
		MinSuperNodes:        minSuperNodes,
		MaxDdAndFingerprints: maxDdAndFingerprints,
		MaxRaptorQSymbols:    maxRaptorQSymbols,
		ExpirationDuration:   expirationDuration,
		MinProcessingTime:    minProcessingTime,
		MaxProcessingTime:    maxProcessingTime,
		SuperNodeFeeShare:    superNodeFeeShare,
		FoundationFeeShare:   foundationFeeShare,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(
		DefaultBaseActionFee,
		DefaultFeePerByte,
		DefaultMaxActionsPerBlock,
		DefaultMinSuperNodes,
		DefaultMaxDdAndFingerprints,
		DefaultMaxRaptorQSymbols,
		DefaultExpirationDuration,
		DefaultMinProcessingTime,
		DefaultMaxProcessingTime,
		DefaultSuperNodeFeeShare,
		DefaultFoundationFeeShare,
	)
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyBaseActionFee, &p.BaseActionFee, validateCoin),
		paramtypes.NewParamSetPair(KeyFeePerByte, &p.FeePerByte, validateCoin),
		paramtypes.NewParamSetPair(KeyMaxActionsPerBlock, &p.MaxActionsPerBlock, validateUint64),
		paramtypes.NewParamSetPair(KeyMinSuperNodes, &p.MinSuperNodes, validateUint64),
		paramtypes.NewParamSetPair(KeyMaxDdAndFingerprints, &p.MaxDdAndFingerprints, validateUint64),
		paramtypes.NewParamSetPair(KeyMaxRaptorQSymbols, &p.MaxRaptorQSymbols, validateUint64),
		paramtypes.NewParamSetPair(KeyExpirationDuration, &p.ExpirationDuration, validateDuration),
		paramtypes.NewParamSetPair(KeyMinProcessingTime, &p.MinProcessingTime, validateDuration),
		paramtypes.NewParamSetPair(KeyMaxProcessingTime, &p.MaxProcessingTime, validateDuration),
		paramtypes.NewParamSetPair(KeySuperNodeFeeShare, &p.SuperNodeFeeShare, validateDecString),
		paramtypes.NewParamSetPair(KeyFoundationFeeShare, &p.FoundationFeeShare, validateDecString),
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateCoin(p.BaseActionFee); err != nil {
		return err
	}

	if err := validateCoin(p.FeePerByte); err != nil {
		return err
	}

	if err := validateUint64(p.MaxActionsPerBlock); err != nil {
		return err
	}

	if err := validateUint64(p.MinSuperNodes); err != nil {
		return err
	}

	if err := validateUint64(p.MaxDdAndFingerprints); err != nil {
		return err
	}

	if err := validateUint64(p.MaxRaptorQSymbols); err != nil {
		return err
	}

	if err := validateDuration(p.ExpirationDuration); err != nil {
		return err
	}

	if err := validateDuration(p.MinProcessingTime); err != nil {
		return err
	}

	if err := validateDuration(p.MaxProcessingTime); err != nil {
		return err
	}

	if err := validateDecString(p.SuperNodeFeeShare); err != nil {
		return err
	}

	if err := validateDecString(p.FoundationFeeShare); err != nil {
		return err
	}

	// Additional validation rules
	if p.MinProcessingTime >= p.MaxProcessingTime {
		return fmt.Errorf("min processing time must be less than max processing time")
	}

	// Check that fee shares sum to 1.0
	superNodeShare, err := math.LegacyNewDecFromStr(p.SuperNodeFeeShare)
	if err != nil {
		return fmt.Errorf("invalid super node fee share: %s", err)
	}

	foundationShare, err := math.LegacyNewDecFromStr(p.FoundationFeeShare)
	if err != nil {
		return fmt.Errorf("invalid foundation fee share: %s", err)
	}

	totalShare := superNodeShare.Add(foundationShare)
	if !totalShare.Equal(math.LegacyOneDec()) {
		return fmt.Errorf("fee shares must sum to 1.0, got %s", totalShare.String())
	}

	return nil
}

// Validation functions

func validateCoin(v interface{}) error {
	coin, ok := v.(sdk.Coin)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if !coin.IsValid() {
		return fmt.Errorf("invalid coin: %s", coin.String())
	}

	return nil
}

func validateUint64(v interface{}) error {
	_, ok := v.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	return nil
}

func validateDuration(v interface{}) error {
	duration, ok := v.(time.Duration)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	if duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	return nil
}

func validateDecString(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", v)
	}

	_, err := math.LegacyNewDecFromStr(str)
	if err != nil {
		return fmt.Errorf("invalid decimal string: %s", err)
	}

	return nil
}
