# Unit Tests: Precisebank (6<>18 Decimal Bridge)

Purpose: verifies precisebank fractional accounting, bank parity behavior, mint/burn transitions, and type-level invariants.

Primary files:
- `app/precisebank_test.go`
- `app/precisebank_fractional_test.go`
- `app/precisebank_mint_burn_behavior_test.go`
- `app/precisebank_mint_burn_parity_test.go`
- `app/precisebank_types_test.go`

| Test | Description |
| --- | --- |
| `TestPreciseBankSplitAndRecomposeBalance` | Verifies extended balance splits into integer+fractional parts and recomposes correctly. |
| `TestPreciseBankSendExtendedCoinBorrowCarry` | Verifies fractional borrow/carry behavior during extended-denom transfers. |
| `TestPreciseBankMintTransferBurnRestoresReserveAndRemainder` | Verifies reserve/remainder bookkeeping round-trips after mint-transfer-burn sequence. |
| `TestPreciseBankSendCoinsErrorParityWithBank` | Verifies send error messages/parity match bank keeper behavior. |
| `TestPreciseBankSendCoinsFromModuleToAccountBlockedRecipientParity` | Verifies blocked-recipient behavior matches bank keeper for module-to-account sends. |
| `TestPreciseBankSendCoinsFromModuleToAccountMissingModulePanicParity` | Verifies missing sender module panic parity with bank keeper. |
| `TestPreciseBankSendCoinsFromAccountToModuleMissingModulePanicParity` | Verifies missing recipient module panic parity with bank keeper. |
| `TestPreciseBankSendCoinsFromModuleToModuleMissingModulePanicParity` | Verifies module-to-module missing-account panic parity with bank keeper. |
| `TestPreciseBankSendCoinsFromModuleToModuleErrorParityWithBank` | Verifies module-to-module error-path parity with bank keeper. |
| `TestPreciseBankSendCoinsFromAccountToPrecisebankModuleBlocked` | Verifies direct sends to precisebank module account are blocked as expected. |
| `TestPreciseBankSendCoinsFromPrecisebankModuleToAccountBlocked` | Verifies restricted sends from precisebank module account are blocked as expected. |
| `TestPreciseBankMintCoinsToPrecisebankModulePanic` | Verifies minting directly into precisebank module account triggers expected panic. |
| `TestPreciseBankBurnCoinsFromPrecisebankModulePanic` | Verifies burning directly from precisebank module account triggers expected panic. |
| `TestPreciseBankRemainderAmountLifecycle` | Verifies remainder amount updates correctly through lifecycle operations. |
| `TestPreciseBankInvalidRemainderAmountPanics` | Verifies invalid remainder values trigger expected panic behavior. |
| `TestPreciseBankReserveAddressHiddenForExtendedDenom` | Verifies reserve internals are hidden behind extended-denom abstractions. |
| `TestPreciseBankGetBalanceAndSpendableCoin` | Verifies balance/spendable responses for extended-denom accounts. |
| `TestPreciseBankSetGetFractionalBalanceMatrix` | Verifies set/get fractional balance matrix across representative values. |
| `TestPreciseBankSetFractionalBalanceEmptyAddrPanics` | Verifies empty address input panics in fractional balance setter. |
| `TestPreciseBankSetFractionalBalanceZeroDeletes` | Verifies setting zero fractional balance removes persisted entry. |
| `TestPreciseBankIterateFractionalBalancesAndAggregateSum` | Verifies iteration and aggregate sum over fractional balance entries. |
| `TestPreciseBankMintCoinsPermissionMatrix` | Verifies mint permission checks by module/denom path. |
| `TestPreciseBankBurnCoinsPermissionMatrix` | Verifies burn permission checks by module/denom path. |
| `TestPreciseBankMintExtendedCoinStateTransitions` | Verifies state transitions for minting extended-denom coins. |
| `TestPreciseBankBurnExtendedCoinStateTransitions` | Verifies state transitions for burning extended-denom coins. |
| `TestPreciseBankMintCoinsStateMatrix` | Verifies mint state matrix across integer/fractional edge cases. |
| `TestPreciseBankMintCoinsMissingModulePanicParity` | Verifies missing-module panic parity for mint path. |
| `TestPreciseBankBurnCoinsMissingModulePanicParity` | Verifies missing-module panic parity for burn path. |
| `TestPreciseBankMintCoinsInvalidCoinsErrorParity` | Verifies invalid coin error parity for mint path. |
| `TestPreciseBankBurnCoinsInvalidCoinsErrorParity` | Verifies invalid coin error parity for burn path. |
| `TestPreciseBankTypesConversionFactorInvariants` | Verifies conversion factor constants and invariants for precisebank math. |
| `TestPreciseBankTypesNewFractionalBalance` | Verifies constructor behavior for fractional balance type. |
| `TestPreciseBankTypesFractionalBalanceValidateMatrix` | Verifies validation matrix for single fractional balance entries. |
| `TestPreciseBankTypesFractionalBalancesValidateMatrix` | Verifies validation matrix for collections of fractional balances. |
| `TestPreciseBankTypesFractionalBalancesSumAndOverflow` | Verifies sum/overflow behavior in fractional balance aggregation. |
| `TestPreciseBankTypesGenesisValidateMatrix` | Verifies precisebank genesis validation matrix. |
| `TestPreciseBankTypesGenesisTotalAmountWithRemainder` | Verifies total-amount computation with remainder in genesis state. |
| `TestPreciseBankTypesFractionalBalanceKey` | Verifies deterministic key derivation for fractional balance store entries. |
| `TestPreciseBankTypesSumExtendedCoin` | Verifies helper math for summing extended-denom coin amounts. |
