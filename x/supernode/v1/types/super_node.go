package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	DefaultP2PPort = "16656"
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

	if s.SupernodeAccount == "" {
		return ErrSupernodeAccountUnspecified
	}

	_, err := sdk.AccAddressFromBech32(s.SupernodeAccount)
	if err != nil {
		return ErrInvalidSupernodeAddress
	}

	// Check if version is not empty
	if s.Version == "" {
		return ErrEmptyVersion
	}

	// Check if state is valid (not unspecified)
	if len(s.States) == 0 {
		return ErrInvalidSuperNodeState
	}
	for _, st := range s.States {
		if st.State == SuperNodeStateUnspecified {
			return ErrInvalidSuperNodeState
		}
	}

	// Check if IP address is not empty
	if len(s.PrevIpAddresses) == 0 || s.PrevIpAddresses[0].Address == "" {
		return ErrEmptyIPAddress
	}

	if s.P2PPort == "" {
		s.P2PPort = DefaultP2PPort
	}

	// Note: timestamps are validated by protobuf (non-nullable)

	return nil
}
