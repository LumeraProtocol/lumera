// Copyright (c) 2024-2025 The Lumera developers
//

// Code generated by MockGen. DO NOT EDIT.
// Source: securekeyx.go

// Package lumeraidmocks is a generated GoMock package.
package lumeraidmocks

import (
	reflect "reflect"

	securekeyx "github.com/LumeraProtocol/lumera/x/lumeraid/securekeyx"
	gomock "github.com/golang/mock/gomock"
)

// MockKeyExchanger is a mock of KeyExchanger interface.
type MockKeyExchanger struct {
	ctrl     *gomock.Controller
	recorder *MockKeyExchangerMockRecorder
}

// MockKeyExchangerMockRecorder is the mock recorder for MockKeyExchanger.
type MockKeyExchangerMockRecorder struct {
	mock *MockKeyExchanger
}

// NewMockKeyExchanger creates a new mock instance.
func NewMockKeyExchanger(ctrl *gomock.Controller) *MockKeyExchanger {
	mock := &MockKeyExchanger{ctrl: ctrl}
	mock.recorder = &MockKeyExchangerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKeyExchanger) EXPECT() *MockKeyExchangerMockRecorder {
	return m.recorder
}

// ComputeSharedSecret mocks base method.
func (m *MockKeyExchanger) ComputeSharedSecret(handshakeBytes, signature []byte) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ComputeSharedSecret", handshakeBytes, signature)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ComputeSharedSecret indicates an expected call of ComputeSharedSecret.
func (mr *MockKeyExchangerMockRecorder) ComputeSharedSecret(handshakeBytes, signature interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ComputeSharedSecret", reflect.TypeOf((*MockKeyExchanger)(nil).ComputeSharedSecret), handshakeBytes, signature)
}

// CreateRequest mocks base method.
func (m *MockKeyExchanger) CreateRequest(remoteAddress string) ([]byte, []byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRequest", remoteAddress)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].([]byte)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateRequest indicates an expected call of CreateRequest.
func (mr *MockKeyExchangerMockRecorder) CreateRequest(remoteAddress interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRequest", reflect.TypeOf((*MockKeyExchanger)(nil).CreateRequest), remoteAddress)
}

// LocalAddress mocks base method.
func (m *MockKeyExchanger) LocalAddress() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LocalAddress")
	ret0, _ := ret[0].(string)
	return ret0
}

// LocalAddress indicates an expected call of LocalAddress.
func (mr *MockKeyExchangerMockRecorder) LocalAddress() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LocalAddress", reflect.TypeOf((*MockKeyExchanger)(nil).LocalAddress))
}

// PeerType mocks base method.
func (m *MockKeyExchanger) PeerType() securekeyx.PeerType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PeerType")
	ret0, _ := ret[0].(securekeyx.PeerType)
	return ret0
}

// PeerType indicates an expected call of PeerType.
func (mr *MockKeyExchangerMockRecorder) PeerType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PeerType", reflect.TypeOf((*MockKeyExchanger)(nil).PeerType))
}
