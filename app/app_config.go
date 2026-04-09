package app

import (
	"time"

	"github.com/cosmos/cosmos-sdk/runtime"
	"cosmossdk.io/depinject/appconfig"
	"google.golang.org/protobuf/types/known/durationpb"

	runtimev1alpha1 "cosmossdk.io/api/cosmos/app/runtime/v1alpha1"
	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	authmodulev1 "cosmossdk.io/api/cosmos/auth/module/v1"
	authzmodulev1 "cosmossdk.io/api/cosmos/authz/module/v1"
	bankmodulev1 "cosmossdk.io/api/cosmos/bank/module/v1"
	circuitmodulev1 "cosmossdk.io/api/cosmos/circuit/module/v1"
	consensusmodulev1 "cosmossdk.io/api/cosmos/consensus/module/v1"
	distrmodulev1 "cosmossdk.io/api/cosmos/distribution/module/v1"
	evidencemodulev1 "cosmossdk.io/api/cosmos/evidence/module/v1"
	feegrantmodulev1 "cosmossdk.io/api/cosmos/feegrant/module/v1"
	genutilmodulev1 "cosmossdk.io/api/cosmos/genutil/module/v1"
	govmodulev1 "cosmossdk.io/api/cosmos/gov/module/v1"
	groupmodulev1 "cosmossdk.io/api/cosmos/group/module/v1"
	mintmodulev1 "cosmossdk.io/api/cosmos/mint/module/v1"
	paramsmodulev1 "cosmossdk.io/api/cosmos/params/module/v1"
	slashingmodulev1 "cosmossdk.io/api/cosmos/slashing/module/v1"
	stakingmodulev1 "cosmossdk.io/api/cosmos/staking/module/v1"
	txconfigv1 "cosmossdk.io/api/cosmos/tx/config/v1"
	upgrademodulev1 "cosmossdk.io/api/cosmos/upgrade/module/v1"
	vestingmodulev1 "cosmossdk.io/api/cosmos/vesting/module/v1"
	_ "cosmossdk.io/x/circuit" // import for side-effects
	circuittypes "cosmossdk.io/x/circuit/types"
	_ "cosmossdk.io/x/evidence" // import for side-effects
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	_ "cosmossdk.io/x/feegrant/module" // import for side-effects
	_ "cosmossdk.io/x/upgrade"    // import for side-effects
	upgradetypes "cosmossdk.io/x/upgrade/types"
	actionmodulev1 "github.com/LumeraProtocol/lumera/x/action/v1/module"
	actionmoduletypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
	auditmodulev1 "github.com/LumeraProtocol/lumera/x/audit/v1/module"
	auditmoduletypes "github.com/LumeraProtocol/lumera/x/audit/v1/types"
	claimmodulev1 "github.com/LumeraProtocol/lumera/x/claim/module"
	claimmoduletypes "github.com/LumeraProtocol/lumera/x/claim/types"
	lumeraidmodulev1 "github.com/LumeraProtocol/lumera/x/lumeraid/module"
	lumeraidmoduletypes "github.com/LumeraProtocol/lumera/x/lumeraid/types"
	supernodemodulev1 "github.com/LumeraProtocol/lumera/x/supernode/v1/module"
	supernodemoduletypes "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config" // import for side-effects
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth/vesting" // import for side-effects
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	_ "github.com/cosmos/cosmos-sdk/x/authz/module" // import for side-effects
	"github.com/cosmos/cosmos-sdk/x/authz"
	_ "github.com/cosmos/cosmos-sdk/x/bank"         // import for side-effects
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	_ "github.com/cosmos/cosmos-sdk/x/consensus" // import for side-effects
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	_ "github.com/cosmos/cosmos-sdk/x/distribution" // import for side-effects
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	_ "github.com/cosmos/cosmos-sdk/x/gov" // import for side-effects
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	_ "github.com/cosmos/cosmos-sdk/x/group/module" // import for side-effects
	"github.com/cosmos/cosmos-sdk/x/group"
	_ "github.com/cosmos/cosmos-sdk/x/mint"         // import for side-effects
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	_ "github.com/cosmos/cosmos-sdk/x/params" // import for side-effects
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	_ "github.com/cosmos/cosmos-sdk/x/slashing" // import for side-effects
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	_ "github.com/cosmos/cosmos-sdk/x/staking" // import for side-effects
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	pfmtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v10/packetforward/types"
	solomachine "github.com/cosmos/ibc-go/v10/modules/light-clients/06-solomachine"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	lcfg "github.com/LumeraProtocol/lumera/config"	
	// this line is used by starport scaffolding # stargate/app/moduleImport
)

var (
	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	genesisModuleOrder = []string{
		consensustypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		group.ModuleName,
		circuittypes.ModuleName,
		// ibc modules
		ibcexported.ModuleName,			// IBC core module
		ibctransfertypes.ModuleName,	// IBC transfer module
		icatypes.ModuleName,			// IBC interchain accounts module (host and controller)
		pfmtypes.ModuleName, 			// IBC packet-forward-middleware
		ibctm.ModuleName,				// IBC Tendermint light client
		solomachine.ModuleName,			// IBC Solo Machine light client
		// chain modules
		lumeraidmoduletypes.ModuleName,
		wasmtypes.ModuleName,
		claimmoduletypes.ModuleName,
		supernodemoduletypes.ModuleName,
		auditmoduletypes.ModuleName,
		actionmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/initGenesis
	}

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	// NOTE: capability module's beginblocker must come before any modules using capabilities (e.g. IBC)
	beginBlockers = []string{
		// cosmos sdk modules
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		authz.ModuleName,
		genutiltypes.ModuleName,
		// ibc modules
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		pfmtypes.ModuleName, // IBC packet-forward-middleware
		// chain modules
		lumeraidmoduletypes.ModuleName,
		wasmtypes.ModuleName,
		claimmoduletypes.ModuleName,
		supernodemoduletypes.ModuleName,
		auditmoduletypes.ModuleName,
		actionmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/beginBlockers
	}

	endBlockers = []string{
		// cosmos sdk modules
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		genutiltypes.ModuleName,
		// ibc modules
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		icatypes.ModuleName,
		pfmtypes.ModuleName, // IBC packet-forward-middleware
		// chain modules
		lumeraidmoduletypes.ModuleName,
		wasmtypes.ModuleName,
		claimmoduletypes.ModuleName,
		supernodemoduletypes.ModuleName,
		auditmoduletypes.ModuleName,
		actionmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/endBlockers
	}

	// NOTE: upgrade module is required to be prioritized
	preBlockers = []string{
		upgradetypes.ModuleName,
		authtypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/preBlockers
	}

	// module account permissions
	moduleAccPerms = []*authmodulev1.ModuleAccountPermission{
		{Account: authtypes.FeeCollectorName},
		{Account: distrtypes.ModuleName},
		{Account: minttypes.ModuleName, Permissions: []string{authtypes.Minter}},
		{Account: stakingtypes.BondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
		{Account: stakingtypes.NotBondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
		{Account: govtypes.ModuleName, Permissions: []string{authtypes.Burner}},
		{Account: ibctransfertypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
		{Account: icatypes.ModuleName},
		{Account: lumeraidmoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner, authtypes.Staking}},
		{Account: wasmtypes.ModuleName, Permissions: []string{authtypes.Burner}},
		{Account: claimmoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner, authtypes.Staking}},
		{Account: supernodemoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner, authtypes.Staking}},
		{Account: actionmoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner, authtypes.Staking}},
		// this line is used by starport scaffolding # stargate/app/maccPerms
	}

	// blocked account addresses
	blockAccAddrs = []string{
		authtypes.FeeCollectorName,
		distrtypes.ModuleName,
		minttypes.ModuleName,
		stakingtypes.BondedPoolName,
		stakingtypes.NotBondedPoolName,
		// We allow the following module accounts to receive funds:
		// govtypes.ModuleName
	}

	// appConfig application configuration (used by depinject)
	appConfig = appconfig.Compose(&appv1alpha1.Config{
		Modules: []*appv1alpha1.ModuleConfig{
			{
				Name: runtime.ModuleName,
				Config: appconfig.WrapAny(&runtimev1alpha1.Module{
					AppName:       Name,
					PreBlockers:   preBlockers,
					BeginBlockers: beginBlockers,
					EndBlockers:   endBlockers,
					InitGenesis:   genesisModuleOrder,
					OverrideStoreKeys: []*runtimev1alpha1.StoreKeyConfig{
						{
							ModuleName: authtypes.ModuleName,
							KvStoreKey: "acc",
						},
					},
					// When ExportGenesis is not specified, the export genesis module order
					// is equal to the init genesis order
					// ExportGenesis: genesisModuleOrder,
					// Uncomment if you want to set a custom migration order here.
					// OrderMigrations: nil,
				}),
			},
			{
				Name: authtypes.ModuleName,
				Config: appconfig.WrapAny(&authmodulev1.Module{
					Bech32Prefix:             lcfg.AccountAddressPrefix,
					ModuleAccountPermissions: moduleAccPerms,
					// Cosmos SDK 0.53.x new feature - unordered transactions 
					// "Fire-and-forget" submission model with timeout_timestamp as TTL/replay protection
					EnableUnorderedTransactions: true,
					// By default modules authority is the governance module. This is configurable with the following:
					// Authority: "group", // A custom module authority can be set using a module name
					// Authority: "cosmos1cwwv22j5ca08ggdv9c2uky355k908694z577tv", // or a specific address
				}),
			},
			{
				Name:   vestingtypes.ModuleName,
				Config: appconfig.WrapAny(&vestingmodulev1.Module{}),
			},
			{
				Name: banktypes.ModuleName,
				Config: appconfig.WrapAny(&bankmodulev1.Module{
					BlockedModuleAccountsOverride: blockAccAddrs,
				}),
			},
			{
				Name:   stakingtypes.ModuleName,
				Config: appconfig.WrapAny(&stakingmodulev1.Module{
					// NOTE: specifying a prefix is only necessary when using bech32 addresses
					// If not specfied, the auth Bech32Prefix appended with "valoper" and "valcons" is used by default
					Bech32PrefixValidator: lcfg.ValidatorAddressPrefix,
					Bech32PrefixConsensus: lcfg.ConsNodeAddressPrefix,
				}),
			},
			{
				Name:   slashingtypes.ModuleName,
				Config: appconfig.WrapAny(&slashingmodulev1.Module{}),
			},
			{
				Name:   paramstypes.ModuleName,
				Config: appconfig.WrapAny(&paramsmodulev1.Module{}),
			},
			{
				Name:   "tx",
				Config: appconfig.WrapAny(&txconfigv1.Config{}),
			},
			{
				Name:   genutiltypes.ModuleName,
				Config: appconfig.WrapAny(&genutilmodulev1.Module{}),
			},
			{
				Name:   authz.ModuleName,
				Config: appconfig.WrapAny(&authzmodulev1.Module{}),
			},
			{
				Name:   upgradetypes.ModuleName,
				Config: appconfig.WrapAny(&upgrademodulev1.Module{}),
			},
			{
				Name:   distrtypes.ModuleName,
				Config: appconfig.WrapAny(&distrmodulev1.Module{}),
			},
			{
				Name:   evidencetypes.ModuleName,
				Config: appconfig.WrapAny(&evidencemodulev1.Module{}),
			},
			{
				Name:   minttypes.ModuleName,
				Config: appconfig.WrapAny(&mintmodulev1.Module{}),
			},
			{
				Name: group.ModuleName,
				Config: appconfig.WrapAny(&groupmodulev1.Module{
					MaxExecutionPeriod: durationpb.New(time.Second * 1209600),
					MaxMetadataLen:     255,
				}),
			},
			{
				Name:   feegrant.ModuleName,
				Config: appconfig.WrapAny(&feegrantmodulev1.Module{}),
			},
			{
				Name:   govtypes.ModuleName,
				Config: appconfig.WrapAny(&govmodulev1.Module{}),
			},
			{
				Name:   consensustypes.ModuleName,
				Config: appconfig.WrapAny(&consensusmodulev1.Module{}),
			},
			{
				Name:   circuittypes.ModuleName,
				Config: appconfig.WrapAny(&circuitmodulev1.Module{}),
			},
			{
				Name:   lumeraidmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&lumeraidmodulev1.Module{}),
			},
			{
				Name:   claimmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&claimmodulev1.Module{}),
			},
			{
				Name:   supernodemoduletypes.ModuleName,
				Config: appconfig.WrapAny(&supernodemodulev1.Module{}),
			},
			{
				Name:   auditmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&auditmodulev1.Module{}),
			},
			{
				Name:   actionmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&actionmodulev1.Module{}),
			},
			// this line is used by starport scaffolding # stargate/app/moduleConfig
		},
	})
)
