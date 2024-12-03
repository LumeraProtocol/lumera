package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidatorAddr converts the validator address string to sdk.ValAddress
func (s *SuperNode) ValidatorAddr() (sdk.ValAddress, error) {
	return sdk.ValAddressFromBech32(s.ValidatorAddress)
}

// Validate performs basic validation of SuperNode fields
func (s *SuperNode) Validate() error {
	// Check if validator address is valid
	if _, err := s.ValidatorAddr(); err != nil {
		return err
	}

	// Check if IP address is not empty
	if s.IpAddress == "" {
		return ErrEmptyIPAddress
	}

	// Check if state is valid (not unspecified)
	if s.State == Unspecified {
		return ErrInvalidSuperNodeState
	}

	// Check if version is not empty
	if s.Version == "" {
		return ErrEmptyVersion
	}

	// Note: timestamps are validated by protobuf (non-nullable)

	return nil
}
