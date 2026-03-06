// Package main provides a devnet test tool for the x/evmigration module.
//
// Modes:
//
//	prepare            — run BEFORE the EVM upgrade to create legacy activity
//	migrate            — run AFTER the EVM upgrade to migrate accounts in batches
//	migrate-validator  — run AFTER the EVM upgrade to migrate the local validator operator
//	cleanup            — remove test keys from the local keyring (based on accounts JSON)
//
// Usage:
//
//	tests_evmigration -mode=prepare -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 -accounts=accounts.json [-funder=validator0]
//	tests_evmigration -mode=migrate -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 -accounts=accounts.json
//	tests_evmigration -mode=migrate-validator -bin=lumerad -rpc=tcp://localhost:26657 -chain-id=lumera-devnet-1 [-funder=validator0]
//	tests_evmigration -mode=cleanup -bin=lumerad -accounts=accounts.json
package main

import (
	"flag"
	"log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type DelegationActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

type UnbondingActivity struct {
	Validator string `json:"validator"`
	Amount    string `json:"amount,omitempty"`
}

type RedelegationActivity struct {
	SrcValidator string `json:"src_validator"`
	DstValidator string `json:"dst_validator"`
	Amount       string `json:"amount,omitempty"`
}

type WithdrawAddressActivity struct {
	Address string `json:"address"`
}

type AuthzGrantActivity struct {
	Grantee string `json:"grantee"`
	MsgType string `json:"msg_type,omitempty"`
}

type AuthzReceiveActivity struct {
	Granter string `json:"granter"`
	MsgType string `json:"msg_type,omitempty"`
}

type FeegrantActivity struct {
	Grantee    string `json:"grantee"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

type FeegrantReceiveActivity struct {
	Granter    string `json:"granter"`
	SpendLimit string `json:"spend_limit,omitempty"`
}

// ClaimActivity records a claim or delayed-claim performed for a legacy account.
type ClaimActivity struct {
	OldAddress string `json:"old_address"`           // Pastel base58 address
	Amount     string `json:"amount,omitempty"`       // e.g. "500000ulume"
	Tier       uint32 `json:"tier,omitempty"`         // 0 = instant claim, 1/2/3 = delayed (6/12/18 months)
	Delayed    bool   `json:"delayed,omitempty"`      // true if this was a delayed-claim
	ClaimKeyID int    `json:"claim_key_id,omitempty"` // index into preseededClaimKeys
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

	Delegations       []DelegationActivity      `json:"delegations,omitempty"`
	Unbondings        []UnbondingActivity       `json:"unbondings,omitempty"`
	Redelegations     []RedelegationActivity    `json:"redelegations,omitempty"`
	WithdrawAddresses []WithdrawAddressActivity `json:"withdraw_addresses,omitempty"`
	AuthzGrants       []AuthzGrantActivity      `json:"authz_grants,omitempty"`
	AuthzAsGrantee    []AuthzReceiveActivity    `json:"authz_as_grantee,omitempty"`
	Feegrants         []FeegrantActivity        `json:"feegrants,omitempty"`
	FeegrantsReceived []FeegrantReceiveActivity `json:"feegrants_received,omitempty"`
	Claims            []ClaimActivity           `json:"claims,omitempty"`

	DelegatedTo       string `json:"delegated_to,omitempty"`
	RedelegatedTo     string `json:"redelegated_to,omitempty"`
	WithdrawAddress   string `json:"withdraw_address,omitempty"`
	AuthzGrantedTo    string `json:"authz_granted_to,omitempty"`
	AuthzReceivedFrom string `json:"authz_received_from,omitempty"`
	FeegrantGrantedTo string `json:"feegrant_granted_to,omitempty"`
	FeegrantFrom      string `json:"feegrant_received_from,omitempty"`

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
	flagMode          = flag.String("mode", "", "prepare or migrate or migrate-validator")
	flagBin           = flag.String("bin", "lumerad", "lumerad binary path")
	flagRPC           = flag.String("rpc", "tcp://localhost:26657", "RPC endpoint")
	flagChainID       = flag.String("chain-id", "lumera-devnet-1", "chain ID")
	flagFile          = flag.String("accounts", "accounts.json", "accounts JSON file path")
	flagHome          = flag.String("home", "", "lumerad home directory (uses default if empty)")
	flagFunder        = flag.String("funder", "", "funder key name (must exist in keyring)")
	flagGas           = flag.String("gas", "500000", "gas limit for transactions (fixed value avoids simulation sequence races)")
	flagGasAdj        = flag.String("gas-adjustment", "1.5", "gas adjustment (only used with --gas=auto)")
	flagGasPrices     = flag.String("gas-prices", "0.025ulume", "gas prices")
	flagEVMCutoverVer = flag.String("evm-cutover-version", "v1.12.0", "lumerad version where non-legacy accounts switch to coin-type 60")
	flagNumAccounts   = flag.Int("num-accounts", 5, "number of legacy accounts to generate")
	flagNumExtra      = flag.Int("num-extra", 5, "number of extra (non-migration) accounts")
	flagAccountTag    = flag.String(
		"account-tag",
		"",
		"optional account name tag for prepare mode (e.g. val1 -> evm_test_val1_000); auto-detected from funder key if empty",
	)
	flagValidatorKeys = flag.String(
		"validator-keys",
		"",
		"validator key name to migrate (default: auto-detect from keyring+staking, requires exactly one local candidate)",
	)
)

func main() {
	flag.Parse()

	// Set bech32 prefixes so SDK address encoding works.
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("lumera", "lumerapub")
	config.SetBech32PrefixForValidator("lumeravaloper", "lumeravaloperpub")
	config.SetBech32PrefixForConsensusNode("lumeravalcons", "lumeravalconspub")

	initNonLegacyCoinType()

	switch *flagMode {
	case "prepare":
		runPrepare()
	case "migrate":
		runMigrate()
	case "migrate-validator":
		runMigrateValidator()
	case "cleanup":
		runCleanup()
	default:
		log.Fatalf("usage: -mode=prepare|migrate|migrate-validator|cleanup")
	}
}
