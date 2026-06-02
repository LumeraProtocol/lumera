package common

// BankSendMsgType is the protobuf type URL for MsgSend, used for authz grants.
const BankSendMsgType = "/cosmos.bank.v1beta1.MsgSend"

// BankSendActivity records a single bank transfer. Bank sends are events rather
// than durable state, so they accumulate one record per transfer.
type BankSendActivity struct {
	To     string `json:"to"`
	Amount string `json:"amount,omitempty"`
	TxHash string `json:"tx_hash,omitempty"`
}

// DelegationActivity records a delegation to a validator.
type DelegationActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

// UnbondingActivity records an unbonding delegation from a validator.
type UnbondingActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

// RedelegationActivity records a redelegation between two validators.
type RedelegationActivity struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       string `json:"amount,omitempty"`
}

// WithdrawAddressActivity records a distribution withdraw-address change.
type WithdrawAddressActivity struct {
	Address string `json:"address"`
}

// AuthzGrantActivity records an authz grant issued to a grantee.
type AuthzGrantActivity struct {
	Grantee string `json:"grantee"`
	MsgType string `json:"msg_type,omitempty"`
}

// AuthzReceiveActivity records an authz grant received from a granter.
type AuthzReceiveActivity struct {
	Granter string `json:"granter"`
	MsgType string `json:"msg_type,omitempty"`
}

// FeegrantActivity records a fee allowance issued to a grantee.
type FeegrantActivity struct {
	Grantee    string `json:"grantee"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

// FeegrantReceiveActivity records a fee allowance received from a granter.
type FeegrantReceiveActivity struct {
	Granter    string `json:"granter"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

// ActionActivity records a CASCADE action created against the chain.
type ActionActivity struct {
	ActionID      string   `json:"action_id"`
	ActionType    string   `json:"action_type,omitempty"`
	Price         string   `json:"price,omitempty"`
	Expiration    string   `json:"expiration,omitempty"`
	State         string   `json:"state,omitempty"`
	Metadata      string   `json:"metadata,omitempty"`
	SuperNodes    []string `json:"super_nodes,omitempty"`
	BlockHeight   int64    `json:"block_height,omitempty"`
	CreatedViaSDK bool     `json:"created_via_sdk,omitempty"`
}

// ActivityLog holds the detailed per-account activity records shared between
// the migration tooling and the activity generator. State-like activities
// (delegations, grants) deduplicate by their target; bank sends accumulate as
// events.
type ActivityLog struct {
	BankSends         []BankSendActivity        `json:"bank_sends,omitempty"`
	Delegations       []DelegationActivity      `json:"delegations,omitempty"`
	Unbondings        []UnbondingActivity       `json:"unbondings,omitempty"`
	Redelegations     []RedelegationActivity    `json:"redelegations,omitempty"`
	WithdrawAddresses []WithdrawAddressActivity `json:"withdraw_addresses,omitempty"`
	AuthzGrants       []AuthzGrantActivity      `json:"authz_grants,omitempty"`
	AuthzAsGrantee    []AuthzReceiveActivity    `json:"authz_as_grantee,omitempty"`
	Feegrants         []FeegrantActivity        `json:"feegrants,omitempty"`
	FeegrantsReceived []FeegrantReceiveActivity `json:"feegrants_received,omitempty"`
	Actions           []ActionActivity          `json:"actions,omitempty"`
}

// AddBankSend records a bank transfer. A send carrying a tx hash is recorded at
// most once (retry safety); a hashless send is always appended.
func (l *ActivityLog) AddBankSend(s BankSendActivity) {
	if s.To == "" {
		return
	}
	if s.TxHash != "" {
		for _, b := range l.BankSends {
			if b.TxHash == s.TxHash {
				return
			}
		}
	}
	l.BankSends = append(l.BankSends, s)
}

// AddDelegation records a delegation, deduplicating by validator address.
func (l *ActivityLog) AddDelegation(validator, amount string) {
	if validator == "" {
		return
	}
	for i := range l.Delegations {
		if l.Delegations[i].Validator == validator {
			if l.Delegations[i].Amount == "" && amount != "" {
				l.Delegations[i].Amount = amount
			}
			return
		}
	}
	l.Delegations = append(l.Delegations, DelegationActivity{Validator: validator, Amount: amount})
}

// AddUnbonding records an unbonding delegation, deduplicating by validator.
func (l *ActivityLog) AddUnbonding(validator, amount string) {
	if validator == "" {
		return
	}
	for i := range l.Unbondings {
		if l.Unbondings[i].Validator == validator {
			if l.Unbondings[i].Amount == "" && amount != "" {
				l.Unbondings[i].Amount = amount
			}
			return
		}
	}
	l.Unbondings = append(l.Unbondings, UnbondingActivity{Validator: validator, Amount: amount})
}

// AddRedelegation records a redelegation, deduplicating by validator pair and
// rejecting self-redelegations.
func (l *ActivityLog) AddRedelegation(src, dst, amount string) {
	if src == "" || dst == "" || src == dst {
		return
	}
	for i := range l.Redelegations {
		if l.Redelegations[i].SrcValidator == src && l.Redelegations[i].DstValidator == dst {
			if l.Redelegations[i].Amount == "" && amount != "" {
				l.Redelegations[i].Amount = amount
			}
			return
		}
	}
	l.Redelegations = append(l.Redelegations, RedelegationActivity{SrcValidator: src, DstValidator: dst, Amount: amount})
}

// AddWithdrawAddress appends a withdraw-address change, skipping consecutive
// duplicates.
func (l *ActivityLog) AddWithdrawAddress(addr string) {
	if addr == "" {
		return
	}
	if n := len(l.WithdrawAddresses); n > 0 && l.WithdrawAddresses[n-1].Address == addr {
		return
	}
	l.WithdrawAddresses = append(l.WithdrawAddresses, WithdrawAddressActivity{Address: addr})
}

// AddAuthzGrant records an authz grant, deduplicating by grantee address.
func (l *ActivityLog) AddAuthzGrant(grantee, msgType string) {
	if grantee == "" {
		return
	}
	for i := range l.AuthzGrants {
		if l.AuthzGrants[i].Grantee == grantee {
			if l.AuthzGrants[i].MsgType == "" && msgType != "" {
				l.AuthzGrants[i].MsgType = msgType
			}
			return
		}
	}
	l.AuthzGrants = append(l.AuthzGrants, AuthzGrantActivity{Grantee: grantee, MsgType: msgType})
}

// AddAuthzAsGrantee records an authz grant received from a granter.
func (l *ActivityLog) AddAuthzAsGrantee(granter, msgType string) {
	if granter == "" {
		return
	}
	for i := range l.AuthzAsGrantee {
		if l.AuthzAsGrantee[i].Granter == granter {
			if l.AuthzAsGrantee[i].MsgType == "" && msgType != "" {
				l.AuthzAsGrantee[i].MsgType = msgType
			}
			return
		}
	}
	l.AuthzAsGrantee = append(l.AuthzAsGrantee, AuthzReceiveActivity{Granter: granter, MsgType: msgType})
}

// AddFeegrant records a fee allowance issued to a grantee, deduplicating by
// grantee address.
func (l *ActivityLog) AddFeegrant(grantee, spendLimit string) {
	if grantee == "" {
		return
	}
	for i := range l.Feegrants {
		if l.Feegrants[i].Grantee == grantee {
			if l.Feegrants[i].SpendLimit == "" && spendLimit != "" {
				l.Feegrants[i].SpendLimit = spendLimit
			}
			return
		}
	}
	l.Feegrants = append(l.Feegrants, FeegrantActivity{Grantee: grantee, SpendLimit: spendLimit})
}

// AddFeegrantAsGrantee records a fee allowance received from a granter.
func (l *ActivityLog) AddFeegrantAsGrantee(granter, spendLimit string) {
	if granter == "" {
		return
	}
	for i := range l.FeegrantsReceived {
		if l.FeegrantsReceived[i].Granter == granter {
			if l.FeegrantsReceived[i].SpendLimit == "" && spendLimit != "" {
				l.FeegrantsReceived[i].SpendLimit = spendLimit
			}
			return
		}
	}
	l.FeegrantsReceived = append(l.FeegrantsReceived, FeegrantReceiveActivity{Granter: granter, SpendLimit: spendLimit})
}

// AddAction records a CASCADE action, deduplicating by action ID. The first
// record for an ID wins; use UpdateActionState to advance its state.
func (l *ActivityLog) AddAction(a ActionActivity) {
	if a.ActionID == "" {
		return
	}
	for _, existing := range l.Actions {
		if existing.ActionID == a.ActionID {
			return
		}
	}
	l.Actions = append(l.Actions, a)
}

// UpdateActionState advances the state of an existing action by ID, returning
// true if a matching action was found.
func (l *ActivityLog) UpdateActionState(actionID, state string) bool {
	for i := range l.Actions {
		if l.Actions[i].ActionID == actionID {
			l.Actions[i].State = state
			return true
		}
	}
	return false
}
