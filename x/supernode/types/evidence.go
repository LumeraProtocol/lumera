package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ReporterAdd converts the reporter string to sdk.AccAddress
func (e *Evidence) ReporterAdd() (sdk.AccAddress, error) {
	return sdk.AccAddressFromBech32(e.ReporterAddress)
}

// GetReportedAdd converts the validator address string to sdk.ValAddress
func (e *Evidence) GetReportedAdd() (sdk.ValAddress, error) {
	return sdk.ValAddressFromBech32(e.ValidatorAddress)
}

// Validate performs basic validation of Evidence fields
func (e *Evidence) Validate() error {
	// Check if reporter address is valid
	if _, err := e.ReporterAdd(); err != nil {
		return err
	}

	// Check if reported address is valid
	if _, err := e.GetReportedAdd(); err != nil {
		return err
	}

	// Check if evidence type is not empty
	if e.EvidenceType == "" {
		return ErrEmptyEvidenceType
	}

	// Check if description is not empty
	if e.Description == "" {
		return ErrEmptyDescription
	}

	return nil
}
