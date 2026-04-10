package keeper

import "github.com/LumeraProtocol/lumera/x/supernode/v1/types"

// SetAuditKeeper injects the audit keeper post-construction to avoid depinject cycles.
func (k *Keeper) SetAuditKeeper(auditKeeper types.AuditKeeper) {
	k.auditKeeper = auditKeeper
}
