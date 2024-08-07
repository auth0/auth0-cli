// Code generated by MockGen. DO NOT EDIT.
// Source: connection.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	management "github.com/auth0/go-auth0/management"
	gomock "github.com/golang/mock/gomock"
)

// MockConnectionAPI is a mock of ConnectionAPI interface.
type MockConnectionAPI struct {
	ctrl     *gomock.Controller
	recorder *MockConnectionAPIMockRecorder
}

// MockConnectionAPIMockRecorder is the mock recorder for MockConnectionAPI.
type MockConnectionAPIMockRecorder struct {
	mock *MockConnectionAPI
}

// NewMockConnectionAPI creates a new mock instance.
func NewMockConnectionAPI(ctrl *gomock.Controller) *MockConnectionAPI {
	mock := &MockConnectionAPI{ctrl: ctrl}
	mock.recorder = &MockConnectionAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConnectionAPI) EXPECT() *MockConnectionAPIMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockConnectionAPI) Create(ctx context.Context, c *management.Connection, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, c}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockConnectionAPIMockRecorder) Create(ctx, c interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, c}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockConnectionAPI)(nil).Create), varargs...)
}

// Delete mocks base method.
func (m *MockConnectionAPI) Delete(ctx context.Context, id string, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockConnectionAPIMockRecorder) Delete(ctx, id interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockConnectionAPI)(nil).Delete), varargs...)
}

// List mocks base method.
func (m *MockConnectionAPI) List(ctx context.Context, opts ...management.RequestOption) (*management.ConnectionList, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(*management.ConnectionList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockConnectionAPIMockRecorder) List(ctx interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockConnectionAPI)(nil).List), varargs...)
}

// Read mocks base method.
func (m *MockConnectionAPI) Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.Connection, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Read", varargs...)
	ret0, _ := ret[0].(*management.Connection)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockConnectionAPIMockRecorder) Read(ctx, id interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockConnectionAPI)(nil).Read), varargs...)
}

// ReadByName mocks base method.
func (m *MockConnectionAPI) ReadByName(ctx context.Context, id string, opts ...management.RequestOption) (*management.Connection, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ReadByName", varargs...)
	ret0, _ := ret[0].(*management.Connection)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadByName indicates an expected call of ReadByName.
func (mr *MockConnectionAPIMockRecorder) ReadByName(ctx, id interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadByName", reflect.TypeOf((*MockConnectionAPI)(nil).ReadByName), varargs...)
}

// Update mocks base method.
func (m *MockConnectionAPI) Update(ctx context.Context, id string, c *management.Connection, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id, c}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockConnectionAPIMockRecorder) Update(ctx, id, c interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id, c}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockConnectionAPI)(nil).Update), varargs...)
}
