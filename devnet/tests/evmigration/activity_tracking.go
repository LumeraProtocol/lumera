// activity_tracking.go provides methods on AccountRecord for recording and
// normalizing on-chain activity (delegations, grants, claims, actions, etc.)
// used during prepare mode and validated after migration.
package main

const (
	// bankSendMsgType is the protobuf type URL for MsgSend, used for authz grants.
	bankSendMsgType = "/cosmos.bank.v1beta1.MsgSend"
)

// normalizeActivityTracking backfills the detailed activity slices from legacy
// scalar fields for backward compatibility with older accounts files.
func (rec *AccountRecord) normalizeActivityTracking() {
	if len(rec.Delegations) == 0 && rec.HasDelegation && rec.DelegatedTo != "" {
		rec.addDelegation(rec.DelegatedTo, "")
	}
	if len(rec.Unbondings) == 0 && rec.HasUnbonding && rec.DelegatedTo != "" {
		rec.addUnbonding(rec.DelegatedTo, "")
	}
	if len(rec.Redelegations) == 0 && rec.HasRedelegation && rec.DelegatedTo != "" && rec.RedelegatedTo != "" {
		rec.addRedelegation(rec.DelegatedTo, rec.RedelegatedTo, "")
	}
	if len(rec.WithdrawAddresses) == 0 && rec.HasThirdPartyWD && rec.WithdrawAddress != "" {
		rec.addWithdrawAddress(rec.WithdrawAddress)
	}
	if len(rec.AuthzGrants) == 0 && rec.HasAuthzGrant && rec.AuthzGrantedTo != "" {
		rec.addAuthzGrant(rec.AuthzGrantedTo, bankSendMsgType)
	}
	if len(rec.AuthzAsGrantee) == 0 && rec.HasAuthzAsGrantee && rec.AuthzReceivedFrom != "" {
		rec.addAuthzAsGrantee(rec.AuthzReceivedFrom, bankSendMsgType)
	}
	if len(rec.Feegrants) == 0 && rec.HasFeegrant && rec.FeegrantGrantedTo != "" {
		rec.addFeegrant(rec.FeegrantGrantedTo, "")
	}
	if len(rec.FeegrantsReceived) == 0 && rec.HasFeegrantGrantee && rec.FeegrantFrom != "" {
		rec.addFeegrantAsGrantee(rec.FeegrantFrom, "")
	}
	rec.refreshLegacyFields()
}

// addDelegation records a delegation to the given validator, deduplicating by validator address.
func (rec *AccountRecord) addDelegation(validator, amount string) {
	if validator == "" {
		return
	}
	for i := range rec.Delegations {
		if rec.Delegations[i].Validator == validator {
			if rec.Delegations[i].Amount == "" && amount != "" {
				rec.Delegations[i].Amount = amount
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Delegations = append(rec.Delegations, DelegationActivity{Validator: validator, Amount: amount})
	rec.refreshLegacyFields()
}

// addUnbonding records an unbonding delegation, deduplicating by validator address.
func (rec *AccountRecord) addUnbonding(validator, amount string) {
	if validator == "" {
		return
	}
	for i := range rec.Unbondings {
		if rec.Unbondings[i].Validator == validator {
			if rec.Unbondings[i].Amount == "" && amount != "" {
				rec.Unbondings[i].Amount = amount
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Unbondings = append(rec.Unbondings, UnbondingActivity{Validator: validator, Amount: amount})
	rec.refreshLegacyFields()
}

// addRedelegation records a redelegation, deduplicating by validator pair.
func (rec *AccountRecord) addRedelegation(srcValidator, dstValidator, amount string) {
	if srcValidator == "" || dstValidator == "" || srcValidator == dstValidator {
		return
	}
	for i := range rec.Redelegations {
		rd := rec.Redelegations[i]
		if rd.SrcValidator == srcValidator && rd.DstValidator == dstValidator {
			if rec.Redelegations[i].Amount == "" && amount != "" {
				rec.Redelegations[i].Amount = amount
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Redelegations = append(rec.Redelegations, RedelegationActivity{
		SrcValidator: srcValidator,
		DstValidator: dstValidator,
		Amount:       amount,
	})
	rec.refreshLegacyFields()
}

// addWithdrawAddress appends a withdraw address change, skipping consecutive duplicates.
func (rec *AccountRecord) addWithdrawAddress(addr string) {
	if addr == "" {
		return
	}
	if n := len(rec.WithdrawAddresses); n > 0 && rec.WithdrawAddresses[n-1].Address == addr {
		rec.refreshLegacyFields()
		return
	}
	rec.WithdrawAddresses = append(rec.WithdrawAddresses, WithdrawAddressActivity{Address: addr})
	rec.refreshLegacyFields()
}

// addAuthzGrant records an authz grant to the given grantee, deduplicating by grantee address.
func (rec *AccountRecord) addAuthzGrant(grantee, msgType string) {
	if grantee == "" {
		return
	}
	for i := range rec.AuthzGrants {
		if rec.AuthzGrants[i].Grantee == grantee {
			if rec.AuthzGrants[i].MsgType == "" && msgType != "" {
				rec.AuthzGrants[i].MsgType = msgType
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.AuthzGrants = append(rec.AuthzGrants, AuthzGrantActivity{Grantee: grantee, MsgType: msgType})
	rec.refreshLegacyFields()
}

// addAuthzAsGrantee records an authz grant received from the given granter.
func (rec *AccountRecord) addAuthzAsGrantee(granter, msgType string) {
	if granter == "" {
		return
	}
	for i := range rec.AuthzAsGrantee {
		if rec.AuthzAsGrantee[i].Granter == granter {
			if rec.AuthzAsGrantee[i].MsgType == "" && msgType != "" {
				rec.AuthzAsGrantee[i].MsgType = msgType
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.AuthzAsGrantee = append(rec.AuthzAsGrantee, AuthzReceiveActivity{Granter: granter, MsgType: msgType})
	rec.refreshLegacyFields()
}

// addFeegrant records a fee grant issued to the given grantee, deduplicating by grantee address.
func (rec *AccountRecord) addFeegrant(grantee, spendLimit string) {
	if grantee == "" {
		return
	}
	for i := range rec.Feegrants {
		if rec.Feegrants[i].Grantee == grantee {
			if rec.Feegrants[i].SpendLimit == "" && spendLimit != "" {
				rec.Feegrants[i].SpendLimit = spendLimit
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Feegrants = append(rec.Feegrants, FeegrantActivity{Grantee: grantee, SpendLimit: spendLimit})
	rec.refreshLegacyFields()
}

// addFeegrantAsGrantee records a fee grant received from the given granter.
func (rec *AccountRecord) addFeegrantAsGrantee(granter, spendLimit string) {
	if granter == "" {
		return
	}
	for i := range rec.FeegrantsReceived {
		if rec.FeegrantsReceived[i].Granter == granter {
			if rec.FeegrantsReceived[i].SpendLimit == "" && spendLimit != "" {
				rec.FeegrantsReceived[i].SpendLimit = spendLimit
			}
			rec.refreshLegacyFields()
			return
		}
	}
	rec.FeegrantsReceived = append(rec.FeegrantsReceived, FeegrantReceiveActivity{Granter: granter, SpendLimit: spendLimit})
	rec.refreshLegacyFields()
}

// addAction records an action with basic fields, deduplicating by action ID.
func (rec *AccountRecord) addAction(actionID, actionType, price string) {
	if actionID == "" {
		return
	}
	for _, a := range rec.Actions {
		if a.ActionID == actionID {
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Actions = append(rec.Actions, ActionActivity{
		ActionID:   actionID,
		ActionType: actionType,
		Price:      price,
	})
	rec.refreshLegacyFields()
}

// addActionFull records an action with all fields populated, deduplicating by action ID.
func (rec *AccountRecord) addActionFull(actionID, actionType, price, expiration, state, metadata string, superNodes []string, blockHeight int64, createdViaSDK bool) {
	if actionID == "" {
		return
	}
	for _, a := range rec.Actions {
		if a.ActionID == actionID {
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Actions = append(rec.Actions, ActionActivity{
		ActionID:      actionID,
		ActionType:    actionType,
		Price:         price,
		Expiration:    expiration,
		State:         state,
		Metadata:      metadata,
		SuperNodes:    superNodes,
		BlockHeight:   blockHeight,
		CreatedViaSDK: createdViaSDK,
	})
	rec.refreshLegacyFields()
}

// updateActionState updates the state field of an existing action record.
func (rec *AccountRecord) updateActionState(actionID, state string) {
	for i := range rec.Actions {
		if rec.Actions[i].ActionID == actionID {
			rec.Actions[i].State = state
			return
		}
	}
}

// addClaim records a claim or delayed-claim activity, deduplicating by old address.
func (rec *AccountRecord) addClaim(oldAddr, amount string, tier uint32, delayed bool, keyID int) {
	if oldAddr == "" {
		return
	}
	for _, c := range rec.Claims {
		if c.OldAddress == oldAddr {
			rec.refreshLegacyFields()
			return
		}
	}
	rec.Claims = append(rec.Claims, ClaimActivity{
		OldAddress: oldAddr,
		Amount:     amount,
		Tier:       tier,
		Delayed:    delayed,
		ClaimKeyID: keyID,
	})
	rec.refreshLegacyFields()
}

// hasDelayedClaim returns true if any recorded claim has a non-zero vesting tier.
func (rec *AccountRecord) hasDelayedClaim() bool {
	for _, claim := range rec.Claims {
		if claim.Delayed || claim.Tier > 0 {
			return true
		}
	}
	return false
}

// hasRecordedAction returns true if the account has any recorded action activity.
func (rec *AccountRecord) hasRecordedAction() bool {
	return len(rec.Actions) > 0 || rec.HasAction
}

// refreshLegacyFields syncs the boolean flags and scalar shorthand fields
// (DelegatedTo, AuthzGrantedTo, etc.) from the detailed activity slices.
func (rec *AccountRecord) refreshLegacyFields() {
	rec.HasDelegation = len(rec.Delegations) > 0 || rec.HasDelegation
	rec.HasUnbonding = len(rec.Unbondings) > 0 || rec.HasUnbonding
	rec.HasRedelegation = len(rec.Redelegations) > 0 || rec.HasRedelegation
	rec.HasAuthzGrant = len(rec.AuthzGrants) > 0 || rec.HasAuthzGrant
	rec.HasAuthzAsGrantee = len(rec.AuthzAsGrantee) > 0 || rec.HasAuthzAsGrantee
	rec.HasFeegrant = len(rec.Feegrants) > 0 || rec.HasFeegrant
	rec.HasFeegrantGrantee = len(rec.FeegrantsReceived) > 0 || rec.HasFeegrantGrantee
	rec.HasThirdPartyWD = len(rec.WithdrawAddresses) > 0 || rec.HasThirdPartyWD
	rec.HasClaim = len(rec.Claims) > 0 || rec.HasClaim
	rec.HasAction = len(rec.Actions) > 0 || rec.HasAction

	if len(rec.Delegations) > 0 {
		rec.DelegatedTo = rec.Delegations[0].Validator
	}
	if len(rec.Redelegations) > 0 {
		if rec.DelegatedTo == "" {
			rec.DelegatedTo = rec.Redelegations[0].SrcValidator
		}
		rec.RedelegatedTo = rec.Redelegations[0].DstValidator
	}
	if n := len(rec.WithdrawAddresses); n > 0 {
		rec.WithdrawAddress = rec.WithdrawAddresses[n-1].Address
	}
	if len(rec.AuthzGrants) > 0 {
		rec.AuthzGrantedTo = rec.AuthzGrants[0].Grantee
	}
	if len(rec.AuthzAsGrantee) > 0 {
		rec.AuthzReceivedFrom = rec.AuthzAsGrantee[0].Granter
	}
	if len(rec.Feegrants) > 0 {
		rec.FeegrantGrantedTo = rec.Feegrants[0].Grantee
	}
	if len(rec.FeegrantsReceived) > 0 {
		rec.FeegrantFrom = rec.FeegrantsReceived[0].Granter
	}
}
