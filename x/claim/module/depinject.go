package claim

import (
	"os"
	"path/filepath"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/LumeraProtocol/lumera/x/claim/keeper"
	"github.com/LumeraProtocol/lumera/x/claim/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

func searchForClaimsFile(appOpts servertypes.AppOptions) (string, error) {
	// Check for explicit override via `--claims-path`
	if val := appOpts.Get(types.FlagClaimsPath); val != nil {
		rawPath := val.(string)

		// Check if it's a directory → append claims.csv
		fi, err := os.Stat(rawPath)
		if err == nil {
			if fi.IsDir() {
				csvPath := filepath.Join(rawPath, types.DefaultClaimsFileName)
				if _, err := os.Stat(csvPath); err == nil {
					return csvPath, nil
				}
				return "", fmt.Errorf("%s not found in provided directory: %s", types.DefaultClaimsFileName, rawPath)
			} else {
				// It's a file → return directly
				return rawPath, nil
			}
		}
		return "", fmt.Errorf("%s provided but path not found: %s", types.FlagClaimsPath, rawPath)
	}

	// Gather candidate fallback paths
	var dirs []string
	
	// App home dir (from --home)
	if appHomeRaw := appOpts.Get(flags.FlagHome); appHomeRaw != nil {
		// Ensure appHomeRaw is a string
		appHome := appHomeRaw.(string)
		dirs = append(dirs,
			filepath.Join(appHome, "config"),
			appHome,
		)		
	}

	// Executable directory
	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs,	filepath.Dir(exePath))
	}

	if userHome, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(userHome, ".lumera", "config"),
			userHome,
			filepath.Join("../../", userHome),
			filepath.Join("../../../", userHome),
		)
	}

	// Try each candidate directory
	for _, dir := range dirs {
		path := filepath.Join(dir, types.DefaultClaimsFileName)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s file not found in any expected locations", types.DefaultClaimsFileName)
}

// ----------------------------------------------------------------------------
// App Wiring Setup
// ----------------------------------------------------------------------------

func init() {
	appconfig.Register(
		&Module{},
		appconfig.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	StoreService  store.KVStoreService
	TStoreService store.TransientStoreService
	Cdc           codec.Codec
	Config        *Module
	Logger        log.Logger
	AppOpts       servertypes.AppOptions

	AccountKeeper types.AccountKeeper
	BankKeeper    types.BankKeeper
}

type ModuleOutputs struct {
	depinject.Out

	ClaimKeeper keeper.Keeper
	Module      appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}
	
	claimsPath, err := searchForClaimsFile(in.AppOpts)
	if err != nil {
		// optionally log or panic if strictly required
		panic(fmt.Sprintf("claims module requires %s: %v", types.DefaultClaimsFileName, err))
	}

	k := keeper.NewKeeper(
		in.Cdc,
		in.StoreService,
		in.TStoreService,
		in.Logger,
		authority.String(),
		in.BankKeeper,
		in.AccountKeeper,
		claimsPath,
	)
	m := NewAppModule(
		in.Cdc,
		k,
		in.AccountKeeper,
		in.BankKeeper,
	)

	return ModuleOutputs{ClaimKeeper: k, Module: m}
}
