// Package main provides a devnet test tool for the x/evmigration module.
//
// Modes:
//
//	prepare            — run BEFORE the EVM upgrade to create legacy activity
//	estimate           — run AFTER the EVM upgrade to query migration estimates only
//	migrate            — run AFTER the EVM upgrade to migrate accounts in batches
//	migrate-validator  — run AFTER the EVM upgrade to migrate the local validator operator
//	cleanup            — remove test keys from the local keyring (based on accounts JSON)
//
// Usage:
//
//	tests_evmigration -mode=prepare -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 -accounts=accounts.json [-funder=validator0]
//	tests_evmigration -mode=estimate -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 -accounts=accounts.json
//	tests_evmigration -mode=migrate -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 -accounts=accounts.json
//	tests_evmigration -mode=migrate-validator -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1
//	tests_evmigration -mode=cleanup -bin=lumerad -accounts=accounts.json
package main

import (
	"flag"
	"log"

	_ "github.com/LumeraProtocol/lumera/config"
)

// DelegationActivity records a staking delegation performed by a legacy account.
type DelegationActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

// UnbondingActivity records an unbonding delegation initiated by a legacy account.
type UnbondingActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

// RedelegationActivity records a redelegation between validators by a legacy account.
type RedelegationActivity struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       string `json:"amount,omitempty"`
}

// WithdrawAddressActivity records a custom distribution withdraw address set by a legacy account.
type WithdrawAddressActivity struct {
	Address string `json:"address"`
}

// AuthzGrantActivity records an authz grant issued by a legacy account (as granter).
type AuthzGrantActivity struct {
	Grantee string `json:"grantee"`
	MsgType string `json:"msg_type,omitempty"`
}

// AuthzReceiveActivity records an authz grant received by a legacy account (as grantee).
type AuthzReceiveActivity struct {
	Granter string `json:"granter"`
	MsgType string `json:"msg_type,omitempty"`
}

// FeegrantActivity records a fee grant issued by a legacy account (as granter).
type FeegrantActivity struct {
	Grantee    string `json:"grantee"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

// FeegrantReceiveActivity records a fee grant received by a legacy account (as grantee).
type FeegrantReceiveActivity struct {
	Granter    string `json:"granter"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

// ClaimActivity records a claim or delayed-claim performed for a legacy account.
type ClaimActivity struct {
	OldAddress string `json:"old_address"`            // Pastel base58 address
	Amount     string `json:"amount,omitempty"`       // e.g. "500000ulume"
	Tier       uint32 `json:"tier,omitempty"`         // 0 = instant claim, 1/2/3 = delayed (6/12/18 months)
	Delayed    bool   `json:"delayed,omitempty"`      // true if this was a delayed-claim
	ClaimKeyID int    `json:"claim_key_id,omitempty"` // index into preseededClaimKeys
}

// ActionActivity records a request-action submitted by a legacy account.
type ActionActivity struct {
	ActionID      string   `json:"action_id"`                 // on-chain action ID returned by request-action tx
	ActionType    string   `json:"action_type"`               // "SENSE" or "CASCADE"
	Price         string   `json:"price,omitempty"`           // e.g. "100000ulume"
	Expiration    string   `json:"expiration,omitempty"`      // unix timestamp string
	State         string   `json:"state,omitempty"`           // e.g. "ACTION_STATE_PENDING"
	Metadata      string   `json:"metadata,omitempty"`        // JSON metadata submitted at creation
	SuperNodes    []string `json:"super_nodes,omitempty"`     // supernode addresses after finalization
	BlockHeight   int64    `json:"block_height,omitempty"`    // block height when action was created
	CreatedViaSDK bool     `json:"created_via_sdk,omitempty"` // true if created using sdk-go
}

// AccountRecord holds a generated test account and its state.
type AccountRecord struct {
	Name       string `json:"name"`
	Mnemonic   string `json:"mnemonic"`
	Address    string `json:"address"`
	PubKeyB64  string `json:"pubkey_b64"` // base64-encoded compressed secp256k1 pubkey
	IsLegacy   bool   `json:"is_legacy"`
	HasBalance bool   `json:"has_balance"`

	// Activity flags (populated in prepare mode).
	HasDelegation      bool `json:"has_delegation,omitempty"`
	HasUnbonding       bool `json:"has_unbonding,omitempty"`
	HasRedelegation    bool `json:"has_redelegation,omitempty"`
	HasAuthzGrant      bool `json:"has_authz_grant,omitempty"`
	HasAuthzAsGrantee  bool `json:"has_authz_as_grantee,omitempty"`
	HasFeegrant        bool `json:"has_feegrant,omitempty"`
	HasFeegrantGrantee bool `json:"has_feegrant_as_grantee,omitempty"`
	HasThirdPartyWD    bool `json:"has_third_party_withdraw,omitempty"`
	HasClaim           bool `json:"has_claim,omitempty"`
	HasAction          bool `json:"has_action,omitempty"`

	Delegations       []DelegationActivity      `json:"delegations,omitempty"`
	Unbondings        []UnbondingActivity       `json:"unbondings,omitempty"`
	Redelegations     []RedelegationActivity    `json:"redelegations,omitempty"`
	WithdrawAddresses []WithdrawAddressActivity `json:"withdraw_addresses,omitempty"`
	AuthzGrants       []AuthzGrantActivity      `json:"authz_grants,omitempty"`
	AuthzAsGrantee    []AuthzReceiveActivity    `json:"authz_as_grantee,omitempty"`
	Feegrants         []FeegrantActivity        `json:"feegrants,omitempty"`
	FeegrantsReceived []FeegrantReceiveActivity `json:"feegrants_received,omitempty"`
	Claims            []ClaimActivity           `json:"claims,omitempty"`
	Actions           []ActionActivity          `json:"actions,omitempty"`

	DelegatedTo       string `json:"delegated_to,omitempty"`
	RedelegatedTo     string `json:"redelegated_to,omitempty"`
	WithdrawAddress   string `json:"withdraw_address,omitempty"`
	AuthzGrantedTo    string `json:"authz_granted_to,omitempty"`
	AuthzReceivedFrom string `json:"authz_received_from,omitempty"`
	FeegrantGrantedTo string `json:"feegrant_granted_to,omitempty"`
	FeegrantFrom      string `json:"feegrant_received_from,omitempty"`

	// Validator fields (populated in prepare mode for validator accounts).
	IsValidator bool   `json:"is_validator,omitempty"`
	Valoper     string `json:"valoper,omitempty"`
	NewValoper  string `json:"new_valoper,omitempty"` // populated after validator migration

	// Pre-migration balance snapshot (populated at migration time).
	PreMigrationBalance int64 `json:"pre_migration_balance,omitempty"`

	// Migration state (populated in migrate mode).
	NewName    string `json:"new_name,omitempty"`
	NewAddress string `json:"new_address,omitempty"`
	Migrated   bool   `json:"migrated,omitempty"`
}

// AccountsFile is the top-level JSON structure persisted between modes.
type AccountsFile struct {
	ChainID    string          `json:"chain_id"`
	CreatedAt  string          `json:"created_at"`
	Funder     string          `json:"funder"`
	Validators []string        `json:"validators"`
	Accounts   []AccountRecord `json:"accounts"`
}

var (
	flagMode          = flag.String("mode", "", "prepare|estimate|migrate|migrate-validator|migrate-all|verify|cleanup")
	flagBin           = flag.String("bin", "lumerad", "lumerad binary path")
	flagRPC           = flag.String("rpc", "tcp://localhost:26657", "RPC endpoint")
	flagGRPC          = flag.String("grpc", "", "gRPC endpoint (default: derived from --rpc host + port 9090)")
	flagChainID       = flag.String("chain-id", "lumera-devnet-1", "chain ID")
	flagFile          = flag.String("accounts", "accounts.json", "accounts JSON file path")
	flagHome          = flag.String("home", "", "lumerad home directory (uses default if empty)")
	flagFunder        = flag.String("funder", "", "funder key name for prepare mode (must exist in keyring)")
	flagGas           = flag.String("gas", "500000", "gas limit for transactions (fixed value avoids simulation sequence races)")
	flagGasAdj        = flag.String("gas-adjustment", "1.5", "gas adjustment (only used with --gas=auto)")
	flagGasPrices     = flag.String("gas-prices", "0.025ulume", "gas prices")
	flagEVMCutoverVer = flag.String("evm-cutover-version", "v1.12.0", "lumerad version where non-legacy accounts switch to coin-type 60")
	flagNumAccounts   = flag.Int("num-accounts", 5, "number of legacy accounts to generate")
	flagNumExtra      = flag.Int("num-extra", 5, "number of extra (non-migration) accounts")
	flagAccountTag    = flag.String(
		"account-tag",
		"",
		"optional account name tag for prepare mode (e.g. val1 -> pre-evm-val1-000); auto-detected from funder key if empty",
	)
	flagValidatorKeys = flag.String(
		"validator-keys",
		"",
		"validator key name to migrate (default: auto-detect from keyring+staking, requires exactly one local candidate)",
	)
)

// main parses flags, detects the runtime coin type, and dispatches to the selected mode.
func main() {
	flag.Parse()

	initNonLegacyCoinType()

	switch *flagMode {
	case "prepare":
		runPrepare()
	case "estimate":
		runEstimate()
	case "migrate":
		runMigrate()
	case "migrate-validator":
		runMigrateValidator()
	case "migrate-all":
		runMigrateAll()
	case "verify":
		runVerify()
	case "cleanup":
		runCleanup()
	default:
		log.Fatalf("usage: -mode=prepare|estimate|migrate|migrate-validator|migrate-all|verify|cleanup")
	}
}
