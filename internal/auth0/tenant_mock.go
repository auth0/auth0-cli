// Code generated by MockGen. DO NOT EDIT.
// Source: tenant.go

// Package auth0 is a generated GoMock package.
package auth0

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	management "gopkg.in/auth0.v5/management"
)

// MockTenantAPI is a mock of TenantAPI interface.
type MockTenantAPI struct {
	ctrl     *gomock.Controller
	recorder *MockTenantAPIMockRecorder
}

// MockTenantAPIMockRecorder is the mock recorder for MockTenantAPI.
type MockTenantAPIMockRecorder struct {
	mock *MockTenantAPI
}

// NewMockTenantAPI creates a new mock instance.
func NewMockTenantAPI(ctrl *gomock.Controller) *MockTenantAPI {
	mock := &MockTenantAPI{ctrl: ctrl}
	mock.recorder = &MockTenantAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTenantAPI) EXPECT() *MockTenantAPIMockRecorder {
	return m.recorder
}

// Read mocks base method.
func (m *MockTenantAPI) Read(opts ...management.RequestOption) (*management.Tenant, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Read", varargs...)
	ret0, _ := ret[0].(*management.Tenant)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockTenantAPIMockRecorder) Read(opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockTenantAPI)(nil).Read), opts...)
}