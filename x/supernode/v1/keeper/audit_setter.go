package keeper

import "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

var globalAuditKeeper types.AuditKeeper

// SetGlobalAuditKeeper wires audit keeper for supernode keepers that cannot be mutably reached
// via depinject interface values.
func SetGlobalAuditKeeper(auditKeeper types.AuditKeeper) {
	globalAuditKeeper = auditKeeper
}

// SetAuditKeeper injects the audit keeper post-construction to avoid depinject cycles.
func (k *Keeper) SetAuditKeeper(auditKeeper types.AuditKeeper) {
	k.auditKeeper = auditKeeper
	SetGlobalAuditKeeper(auditKeeper)
}
