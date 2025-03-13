package types

import (
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
)

// ActionWithMetadataID extends the Action type with a MetadataID field for testing compatibility
// This is used only for testing to support the existing tests that still use the MetadataID field approach
type ActionWithMetadataID struct {
	*actionapi.Action
	MetadataID string
}

// NewActionWithMetadataID creates a new ActionWithMetadataID from an Action
func NewActionWithMetadataID(action *actionapi.Action) *ActionWithMetadataID {
	// If the action has metadata, use it as MetadataID for backward compatibility
	metadataID := ""
	if len(action.Metadata) > 0 {
		metadataID = string(action.Metadata)
	}

	return &ActionWithMetadataID{
		Action:     action,
		MetadataID: metadataID,
	}
}

// CopyToAction copies the MetadataID to Metadata for compatibility
func (a *ActionWithMetadataID) CopyToAction() *actionapi.Action {
	if a.MetadataID != "" && len(a.Metadata) == 0 {
		a.Metadata = []byte(a.MetadataID)
	}
	return a.Action
}
