# Integration Tests: IBC ERC20 Middleware

Purpose: validates ERC20 middleware behavior on ICS20 receive and edge-case handling for mapping registration.
Suite: `tests/integration/evm/ibc/suite_test.go`

| Test | Description |
| --- | --- |
| `RegistersTokenPairOnRecv` | Ensures valid incoming ICS20 transfers auto-register ERC20 token pairs/maps. |
| `NoRegistrationWhenDisabled` | Ensures registration is skipped when ERC20 middleware feature is disabled. |
| `NoRegistrationForInvalidReceiver` | Ensures invalid receiver payloads do not create token mappings. |
| `DenomCollisionKeepsExistingMap` | Ensures existing denom-map collisions are preserved and not overwritten. |
| `RoundTripTransfer` | Full IBC forward+reverse transfer with ERC20 registration, BalanceOf, and balance restore. |
| `SecondaryDenomRegistration` | Verifies non-native denom (ufoo) gets ERC20 auto-registration and dynamic precompile. |
| `TransferBackBurnsVoucher` | Verifies return transfer zeros bank and ERC20 balances while token pair persists. |
