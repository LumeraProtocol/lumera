package main

import (
	"fmt"
	"regexp"
	"strconv"

	"gen/tests/common"
)

// schemaVersion identifies the gen-activity registry layout. v2 adds multisig
// account records (AccountRecord.Multisig). v1 files are not supported and must
// be regenerated.
const schemaVersion = 2

// MultisigInfo describes a generated K-of-N multisig account: its member key
// names, signing threshold (K), and total signer count (N).
type MultisigInfo struct {
	MemberNames []string `json:"member_names"`
	Threshold   int      `json:"threshold"`
	Signers     int      `json:"signers"`
}

// Migration outcome statuses recorded in MigrationInfo.Status.
const (
	MigrationStatusMigrated        = "migrated"
	MigrationStatusAlreadyMigrated = "already_migrated"
	MigrationStatusSkipped         = "skipped"
	MigrationStatusFailed          = "failed"
)

// MigrationInfo records the result of migrating an account to its EVM-compatible
// counterpart. It is populated by migrate mode and is absent on records that
// have never been through migration.
type MigrationInfo struct {
	NewName    string `json:"new_name,omitempty"`
	NewAddress string `json:"new_address,omitempty"`
	TxHash     string `json:"tx_hash,omitempty"`
	Height     int64  `json:"height,omitempty"`
	MigratedAt string `json:"migrated_at,omitempty"`
	Status     string `json:"status,omitempty"` // migrated, already_migrated, skipped, failed
	Error      string `json:"error,omitempty"`
}

// VestingInfo describes a vesting or permanent-locked account. Type is one of
// the common.VestingType values ("continuous", "delayed", "permanent_locked").
// EndTime is the unix unlock time (0 for permanent_locked).
type VestingInfo struct {
	Type         string `json:"type"`
	EndTime      int64  `json:"end_time,omitempty"`
	LockedAmount string `json:"locked_amount"`
}

// AccountRecord is a gen-activity account: the shared identity and activity log
// plus funding/timestamp metadata owned by this tool.
type AccountRecord struct {
	common.AccountIdentity
	common.ActivityLog

	HasBalance bool           `json:"has_balance,omitempty"`
	Funded     bool           `json:"funded,omitempty"`
	CreatedAt  string         `json:"created_at,omitempty"`
	UpdatedAt  string         `json:"updated_at,omitempty"`
	Multisig   *MultisigInfo  `json:"multisig,omitempty"`
	Vesting    *VestingInfo   `json:"vesting,omitempty"`
	Migration  *MigrationInfo `json:"migration,omitempty"`
}

// ActivityRegistry is the gen-activity-owned top-level registry envelope. It is
// deliberately distinct from evmigration's AccountsFile.
type ActivityRegistry struct {
	SchemaVersion int              `json:"schema_version"`
	ChainID       string           `json:"chain_id"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
	FunderKey     string           `json:"funder_key"`
	FunderAddress string           `json:"funder_address"`
	KeyStyle      string           `json:"key_style"`
	Validators    []string         `json:"validators"`
	Accounts      []*AccountRecord `json:"accounts"`
}

// NewRegistry creates an empty registry envelope stamped with the given
// creation time.
func NewRegistry(chainID, funderKey, funderAddr, keyStyle, createdAt string) *ActivityRegistry {
	return &ActivityRegistry{
		SchemaVersion: schemaVersion,
		ChainID:       chainID,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
		FunderKey:     funderKey,
		FunderAddress: funderAddr,
		KeyStyle:      keyStyle,
	}
}

// LoadRegistry reads a registry from path. A missing file surfaces an
// os.IsNotExist error so callers can distinguish "create new" from "corrupt";
// an unparseable file is a hard error so reruns never silently overwrite it.
func LoadRegistry(path string) (*ActivityRegistry, error) {
	var reg ActivityRegistry
	if err := common.ReadJSON(path, &reg); err != nil {
		return nil, err
	}
	if reg.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("unsupported gen-activity registry schema_version %d (want %d)", reg.SchemaVersion, schemaVersion)
	}
	return &reg, nil
}

// Save persists the registry atomically, stamping the given update time.
func (r *ActivityRegistry) Save(path, updatedAt string) error {
	r.UpdatedAt = updatedAt
	return common.AtomicWriteJSON(path, r)
}

// UpsertAccount updates an existing account in place when its name and address
// match, otherwise appends it.
func (r *ActivityRegistry) UpsertAccount(rec *AccountRecord) {
	for i, existing := range r.Accounts {
		if existing.Name == rec.Name && existing.Address == rec.Address {
			r.Accounts[i] = rec
			return
		}
	}
	r.Accounts = append(r.Accounts, rec)
}

// AllocateNames returns n fresh account names of the form "<prefix>-NNNN",
// continuing past the highest existing index that matches the prefix so reruns
// never collide with previously generated accounts.
func (r *ActivityRegistry) AllocateNames(prefix string, n int) []string {
	pattern := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + `-(\d+)$`)
	highest := 0
	for _, acct := range r.Accounts {
		if m := pattern.FindStringSubmatch(acct.Name); m != nil {
			if idx, err := strconv.Atoi(m[1]); err == nil && idx > highest {
				highest = idx
			}
		}
	}
	names := make([]string, n)
	for i := range n {
		names[i] = fmt.Sprintf("%s-%04d", prefix, highest+i+1)
	}
	return names
}
