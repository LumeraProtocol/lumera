package app

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/client/flags"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	"github.com/LumeraProtocol/lumera/app/params"
)

// MakeEncodingConfig creates a new EncodingConfig with all modules registered. For testing only
func MakeEncodingConfig(t testing.TB) params.EncodingConfig {
	t.Helper()
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	// note, this is not necessary when using app wiring, as depinject can be directly used (see root_v2.go)
	appOpts := simtestutil.AppOptionsMap{
		flags.FlagHome:  t.TempDir(),
		FlagWasmHomeDir: t.TempDir(),
	}
	tempApp := New(log.NewNopLogger(), dbm.NewMemDB(), nil, true,
		appOpts,
		GetDefaultWasmOptions())
	return makeEncodingConfig(tempApp)
}

func makeEncodingConfig(tempApp *App) params.EncodingConfig {
	encodingConfig := params.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.TxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	return encodingConfig
}
