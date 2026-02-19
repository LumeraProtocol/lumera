//go:build test
// +build test

package ibc_test

import "testing"

// TestIBCERC20MiddlewareSuite groups ERC20 IBC middleware integration checks.
// Each subtest provisions its own coordinator/path fixture to keep state isolated.
func TestIBCERC20MiddlewareSuite(t *testing.T) {
	t.Run("RegistersTokenPairOnRecv", func(t *testing.T) {
		testIBCERC20MiddlewareRegistersTokenPairOnRecv(t)
	})
	t.Run("NoRegistrationWhenDisabled", func(t *testing.T) {
		testIBCERC20MiddlewareNoRegistrationWhenDisabled(t)
	})
	t.Run("NoRegistrationForInvalidReceiver", func(t *testing.T) {
		testIBCERC20MiddlewareNoRegistrationForInvalidReceiver(t)
	})
	t.Run("DenomCollisionKeepsExistingMap", func(t *testing.T) {
		testIBCERC20MiddlewareDenomCollisionKeepsExistingMap(t)
	})
}
