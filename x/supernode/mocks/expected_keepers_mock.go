// Code generated by MockGen. DO NOT EDIT.
// Source: expected_keepers.go

// Package supernodemocks is a generated GoMock package.
package supernodemocks

import (
	context "context"
	reflect "reflect"
	time "time"

	address "cosmossdk.io/core/address"
	math "cosmossdk.io/math"
	types "github.com/cosmos/cosmos-sdk/types"
	types0 "github.com/cosmos/cosmos-sdk/x/staking/types"
	gomock "github.com/golang/mock/gomock"
)

// MockSupernodeKeeper is a mock of SupernodeKeeper interface.
type MockSupernodeKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockSupernodeKeeperMockRecorder
}

// MockSupernodeKeeperMockRecorder is the mock recorder for MockSupernodeKeeper.
type MockSupernodeKeeperMockRecorder struct {
	mock *MockSupernodeKeeper
}

// NewMockSupernodeKeeper creates a new mock instance.
func NewMockSupernodeKeeper(ctrl *gomock.Controller) *MockSupernodeKeeper {
	mock := &MockSupernodeKeeper{ctrl: ctrl}
	mock.recorder = &MockSupernodeKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSupernodeKeeper) EXPECT() *MockSupernodeKeeperMockRecorder {
	return m.recorder
}

// DisableSuperNode mocks base method.
func (m *MockSupernodeKeeper) DisableSuperNode(ctx types.Context, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DisableSuperNode", ctx, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// DisableSuperNode indicates an expected call of DisableSuperNode.
func (mr *MockSupernodeKeeperMockRecorder) DisableSuperNode(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisableSuperNode", reflect.TypeOf((*MockSupernodeKeeper)(nil).DisableSuperNode), ctx, valAddr)
}

// EnableSuperNode mocks base method.
func (m *MockSupernodeKeeper) EnableSuperNode(ctx types.Context, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnableSuperNode", ctx, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// EnableSuperNode indicates an expected call of EnableSuperNode.
func (mr *MockSupernodeKeeperMockRecorder) EnableSuperNode(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableSuperNode", reflect.TypeOf((*MockSupernodeKeeper)(nil).EnableSuperNode), ctx, valAddr)
}

// IsEligibleAndNotJailedValidator mocks base method.
func (m *MockSupernodeKeeper) IsEligibleAndNotJailedValidator(ctx types.Context, valAddr types.ValAddress) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsEligibleAndNotJailedValidator", ctx, valAddr)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsEligibleAndNotJailedValidator indicates an expected call of IsEligibleAndNotJailedValidator.
func (mr *MockSupernodeKeeperMockRecorder) IsEligibleAndNotJailedValidator(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsEligibleAndNotJailedValidator", reflect.TypeOf((*MockSupernodeKeeper)(nil).IsEligibleAndNotJailedValidator), ctx, valAddr)
}

// IsSuperNodeActive mocks base method.
func (m *MockSupernodeKeeper) IsSuperNodeActive(ctx types.Context, valAddr types.ValAddress) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSuperNodeActive", ctx, valAddr)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsSuperNodeActive indicates an expected call of IsSuperNodeActive.
func (mr *MockSupernodeKeeperMockRecorder) IsSuperNodeActive(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSuperNodeActive", reflect.TypeOf((*MockSupernodeKeeper)(nil).IsSuperNodeActive), ctx, valAddr)
}

// MockStakingKeeper is a mock of StakingKeeper interface.
type MockStakingKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockStakingKeeperMockRecorder
}

// MockStakingKeeperMockRecorder is the mock recorder for MockStakingKeeper.
type MockStakingKeeperMockRecorder struct {
	mock *MockStakingKeeper
}

// NewMockStakingKeeper creates a new mock instance.
func NewMockStakingKeeper(ctrl *gomock.Controller) *MockStakingKeeper {
	mock := &MockStakingKeeper{ctrl: ctrl}
	mock.recorder = &MockStakingKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStakingKeeper) EXPECT() *MockStakingKeeperMockRecorder {
	return m.recorder
}

// ConsensusAddressCodec mocks base method.
func (m *MockStakingKeeper) ConsensusAddressCodec() address.Codec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConsensusAddressCodec")
	ret0, _ := ret[0].(address.Codec)
	return ret0
}

// ConsensusAddressCodec indicates an expected call of ConsensusAddressCodec.
func (mr *MockStakingKeeperMockRecorder) ConsensusAddressCodec() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConsensusAddressCodec", reflect.TypeOf((*MockStakingKeeper)(nil).ConsensusAddressCodec))
}

// Delegation mocks base method.
func (m *MockStakingKeeper) Delegation(ctx context.Context, delAddr types.AccAddress, valAddr types.ValAddress) (types0.DelegationI, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delegation", ctx, delAddr, valAddr)
	ret0, _ := ret[0].(types0.DelegationI)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delegation indicates an expected call of Delegation.
func (mr *MockStakingKeeperMockRecorder) Delegation(ctx, delAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delegation", reflect.TypeOf((*MockStakingKeeper)(nil).Delegation), ctx, delAddr, valAddr)
}

// Validator mocks base method.
func (m *MockStakingKeeper) Validator(arg0 context.Context, arg1 types.ValAddress) (types0.ValidatorI, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validator", arg0, arg1)
	ret0, _ := ret[0].(types0.ValidatorI)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Validator indicates an expected call of Validator.
func (mr *MockStakingKeeperMockRecorder) Validator(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validator", reflect.TypeOf((*MockStakingKeeper)(nil).Validator), arg0, arg1)
}

// ValidatorByConsAddr mocks base method.
func (m *MockStakingKeeper) ValidatorByConsAddr(arg0 context.Context, arg1 types.ConsAddress) (types0.ValidatorI, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidatorByConsAddr", arg0, arg1)
	ret0, _ := ret[0].(types0.ValidatorI)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidatorByConsAddr indicates an expected call of ValidatorByConsAddr.
func (mr *MockStakingKeeperMockRecorder) ValidatorByConsAddr(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorByConsAddr", reflect.TypeOf((*MockStakingKeeper)(nil).ValidatorByConsAddr), arg0, arg1)
}

// MockSlashingKeeper is a mock of SlashingKeeper interface.
type MockSlashingKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockSlashingKeeperMockRecorder
}

// MockSlashingKeeperMockRecorder is the mock recorder for MockSlashingKeeper.
type MockSlashingKeeperMockRecorder struct {
	mock *MockSlashingKeeper
}

// NewMockSlashingKeeper creates a new mock instance.
func NewMockSlashingKeeper(ctrl *gomock.Controller) *MockSlashingKeeper {
	mock := &MockSlashingKeeper{ctrl: ctrl}
	mock.recorder = &MockSlashingKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSlashingKeeper) EXPECT() *MockSlashingKeeperMockRecorder {
	return m.recorder
}

// IsTombstoned mocks base method.
func (m *MockSlashingKeeper) IsTombstoned(arg0 context.Context, arg1 types.ConsAddress) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsTombstoned", arg0, arg1)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsTombstoned indicates an expected call of IsTombstoned.
func (mr *MockSlashingKeeperMockRecorder) IsTombstoned(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsTombstoned", reflect.TypeOf((*MockSlashingKeeper)(nil).IsTombstoned), arg0, arg1)
}

// Jail mocks base method.
func (m *MockSlashingKeeper) Jail(arg0 context.Context, arg1 types.ConsAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Jail", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Jail indicates an expected call of Jail.
func (mr *MockSlashingKeeperMockRecorder) Jail(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Jail", reflect.TypeOf((*MockSlashingKeeper)(nil).Jail), arg0, arg1)
}

// JailUntil mocks base method.
func (m *MockSlashingKeeper) JailUntil(arg0 context.Context, arg1 types.ConsAddress, arg2 time.Time) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JailUntil", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// JailUntil indicates an expected call of JailUntil.
func (mr *MockSlashingKeeperMockRecorder) JailUntil(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JailUntil", reflect.TypeOf((*MockSlashingKeeper)(nil).JailUntil), arg0, arg1, arg2)
}

// Slash mocks base method.
func (m *MockSlashingKeeper) Slash(ctx context.Context, consAddr types.ConsAddress, fraction math.LegacyDec, power, distributionHeight int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Slash", ctx, consAddr, fraction, power, distributionHeight)
	ret0, _ := ret[0].(error)
	return ret0
}

// Slash indicates an expected call of Slash.
func (mr *MockSlashingKeeperMockRecorder) Slash(ctx, consAddr, fraction, power, distributionHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Slash", reflect.TypeOf((*MockSlashingKeeper)(nil).Slash), ctx, consAddr, fraction, power, distributionHeight)
}

// MockAccountKeeper is a mock of AccountKeeper interface.
type MockAccountKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockAccountKeeperMockRecorder
}

// MockAccountKeeperMockRecorder is the mock recorder for MockAccountKeeper.
type MockAccountKeeperMockRecorder struct {
	mock *MockAccountKeeper
}

// NewMockAccountKeeper creates a new mock instance.
func NewMockAccountKeeper(ctrl *gomock.Controller) *MockAccountKeeper {
	mock := &MockAccountKeeper{ctrl: ctrl}
	mock.recorder = &MockAccountKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAccountKeeper) EXPECT() *MockAccountKeeperMockRecorder {
	return m.recorder
}

// GetAccount mocks base method.
func (m *MockAccountKeeper) GetAccount(arg0 context.Context, arg1 types.AccAddress) types.AccountI {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccount", arg0, arg1)
	ret0, _ := ret[0].(types.AccountI)
	return ret0
}

// GetAccount indicates an expected call of GetAccount.
func (mr *MockAccountKeeperMockRecorder) GetAccount(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccount", reflect.TypeOf((*MockAccountKeeper)(nil).GetAccount), arg0, arg1)
}

// MockBankKeeper is a mock of BankKeeper interface.
type MockBankKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockBankKeeperMockRecorder
}

// MockBankKeeperMockRecorder is the mock recorder for MockBankKeeper.
type MockBankKeeperMockRecorder struct {
	mock *MockBankKeeper
}

// NewMockBankKeeper creates a new mock instance.
func NewMockBankKeeper(ctrl *gomock.Controller) *MockBankKeeper {
	mock := &MockBankKeeper{ctrl: ctrl}
	mock.recorder = &MockBankKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBankKeeper) EXPECT() *MockBankKeeperMockRecorder {
	return m.recorder
}

// GetBalance mocks base method.
func (m *MockBankKeeper) GetBalance(ctx context.Context, addr types.AccAddress, denom string) types.Coin {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBalance", ctx, addr, denom)
	ret0, _ := ret[0].(types.Coin)
	return ret0
}

// GetBalance indicates an expected call of GetBalance.
func (mr *MockBankKeeperMockRecorder) GetBalance(ctx, addr, denom interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBalance", reflect.TypeOf((*MockBankKeeper)(nil).GetBalance), ctx, addr, denom)
}

// SpendableCoins mocks base method.
func (m *MockBankKeeper) SpendableCoins(arg0 context.Context, arg1 types.AccAddress) types.Coins {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SpendableCoins", arg0, arg1)
	ret0, _ := ret[0].(types.Coins)
	return ret0
}

// SpendableCoins indicates an expected call of SpendableCoins.
func (mr *MockBankKeeperMockRecorder) SpendableCoins(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SpendableCoins", reflect.TypeOf((*MockBankKeeper)(nil).SpendableCoins), arg0, arg1)
}

// MockStakingHooks is a mock of StakingHooks interface.
type MockStakingHooks struct {
	ctrl     *gomock.Controller
	recorder *MockStakingHooksMockRecorder
}

// MockStakingHooksMockRecorder is the mock recorder for MockStakingHooks.
type MockStakingHooksMockRecorder struct {
	mock *MockStakingHooks
}

// NewMockStakingHooks creates a new mock instance.
func NewMockStakingHooks(ctrl *gomock.Controller) *MockStakingHooks {
	mock := &MockStakingHooks{ctrl: ctrl}
	mock.recorder = &MockStakingHooksMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStakingHooks) EXPECT() *MockStakingHooksMockRecorder {
	return m.recorder
}

// AfterDelegationModified mocks base method.
func (m *MockStakingHooks) AfterDelegationModified(ctx context.Context, delAddr types.AccAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AfterDelegationModified", ctx, delAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// AfterDelegationModified indicates an expected call of AfterDelegationModified.
func (mr *MockStakingHooksMockRecorder) AfterDelegationModified(ctx, delAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AfterDelegationModified", reflect.TypeOf((*MockStakingHooks)(nil).AfterDelegationModified), ctx, delAddr, valAddr)
}

// AfterValidatorBeginUnbonding mocks base method.
func (m *MockStakingHooks) AfterValidatorBeginUnbonding(ctx context.Context, consAddr types.ConsAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AfterValidatorBeginUnbonding", ctx, consAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// AfterValidatorBeginUnbonding indicates an expected call of AfterValidatorBeginUnbonding.
func (mr *MockStakingHooksMockRecorder) AfterValidatorBeginUnbonding(ctx, consAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AfterValidatorBeginUnbonding", reflect.TypeOf((*MockStakingHooks)(nil).AfterValidatorBeginUnbonding), ctx, consAddr, valAddr)
}

// AfterValidatorBonded mocks base method.
func (m *MockStakingHooks) AfterValidatorBonded(ctx context.Context, consAddr types.ConsAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AfterValidatorBonded", ctx, consAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// AfterValidatorBonded indicates an expected call of AfterValidatorBonded.
func (mr *MockStakingHooksMockRecorder) AfterValidatorBonded(ctx, consAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AfterValidatorBonded", reflect.TypeOf((*MockStakingHooks)(nil).AfterValidatorBonded), ctx, consAddr, valAddr)
}

// AfterValidatorCreated mocks base method.
func (m *MockStakingHooks) AfterValidatorCreated(ctx context.Context, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AfterValidatorCreated", ctx, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// AfterValidatorCreated indicates an expected call of AfterValidatorCreated.
func (mr *MockStakingHooksMockRecorder) AfterValidatorCreated(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AfterValidatorCreated", reflect.TypeOf((*MockStakingHooks)(nil).AfterValidatorCreated), ctx, valAddr)
}

// AfterValidatorRemoved mocks base method.
func (m *MockStakingHooks) AfterValidatorRemoved(ctx context.Context, consAddr types.ConsAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AfterValidatorRemoved", ctx, consAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// AfterValidatorRemoved indicates an expected call of AfterValidatorRemoved.
func (mr *MockStakingHooksMockRecorder) AfterValidatorRemoved(ctx, consAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AfterValidatorRemoved", reflect.TypeOf((*MockStakingHooks)(nil).AfterValidatorRemoved), ctx, consAddr, valAddr)
}

// BeforeDelegationCreated mocks base method.
func (m *MockStakingHooks) BeforeDelegationCreated(ctx context.Context, delAddr types.AccAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BeforeDelegationCreated", ctx, delAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// BeforeDelegationCreated indicates an expected call of BeforeDelegationCreated.
func (mr *MockStakingHooksMockRecorder) BeforeDelegationCreated(ctx, delAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BeforeDelegationCreated", reflect.TypeOf((*MockStakingHooks)(nil).BeforeDelegationCreated), ctx, delAddr, valAddr)
}

// BeforeDelegationRemoved mocks base method.
func (m *MockStakingHooks) BeforeDelegationRemoved(ctx context.Context, delAddr types.AccAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BeforeDelegationRemoved", ctx, delAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// BeforeDelegationRemoved indicates an expected call of BeforeDelegationRemoved.
func (mr *MockStakingHooksMockRecorder) BeforeDelegationRemoved(ctx, delAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BeforeDelegationRemoved", reflect.TypeOf((*MockStakingHooks)(nil).BeforeDelegationRemoved), ctx, delAddr, valAddr)
}

// BeforeDelegationSharesModified mocks base method.
func (m *MockStakingHooks) BeforeDelegationSharesModified(ctx context.Context, delAddr types.AccAddress, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BeforeDelegationSharesModified", ctx, delAddr, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// BeforeDelegationSharesModified indicates an expected call of BeforeDelegationSharesModified.
func (mr *MockStakingHooksMockRecorder) BeforeDelegationSharesModified(ctx, delAddr, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BeforeDelegationSharesModified", reflect.TypeOf((*MockStakingHooks)(nil).BeforeDelegationSharesModified), ctx, delAddr, valAddr)
}

// BeforeValidatorModified mocks base method.
func (m *MockStakingHooks) BeforeValidatorModified(ctx context.Context, valAddr types.ValAddress) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BeforeValidatorModified", ctx, valAddr)
	ret0, _ := ret[0].(error)
	return ret0
}

// BeforeValidatorModified indicates an expected call of BeforeValidatorModified.
func (mr *MockStakingHooksMockRecorder) BeforeValidatorModified(ctx, valAddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BeforeValidatorModified", reflect.TypeOf((*MockStakingHooks)(nil).BeforeValidatorModified), ctx, valAddr)
}

// BeforeValidatorSlashed mocks base method.
func (m *MockStakingHooks) BeforeValidatorSlashed(ctx context.Context, valAddr types.ValAddress, fraction math.LegacyDec) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BeforeValidatorSlashed", ctx, valAddr, fraction)
	ret0, _ := ret[0].(error)
	return ret0
}

// BeforeValidatorSlashed indicates an expected call of BeforeValidatorSlashed.
func (mr *MockStakingHooksMockRecorder) BeforeValidatorSlashed(ctx, valAddr, fraction interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BeforeValidatorSlashed", reflect.TypeOf((*MockStakingHooks)(nil).BeforeValidatorSlashed), ctx, valAddr, fraction)
}
