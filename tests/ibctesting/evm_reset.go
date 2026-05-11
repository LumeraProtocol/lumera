package ibctesting

import appevm "github.com/LumeraProtocol/lumera/app/evm"

// resetEVMGlobalState delegates to app/evm.ResetGlobalState, which handles
// build-tag dispatch internally (no-op in production, real reset in test builds).
func resetEVMGlobalState() {
	appevm.ResetGlobalState()
}
