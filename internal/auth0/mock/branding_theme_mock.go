// Code generated by MockGen. DO NOT EDIT.
// Source: branding_theme.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	management "github.com/auth0/go-auth0/management"
	gomock "github.com/golang/mock/gomock"
)

// MockBrandingThemeAPI is a mock of BrandingThemeAPI interface.
type MockBrandingThemeAPI struct {
	ctrl     *gomock.Controller
	recorder *MockBrandingThemeAPIMockRecorder
}

// MockBrandingThemeAPIMockRecorder is the mock recorder for MockBrandingThemeAPI.
type MockBrandingThemeAPIMockRecorder struct {
	mock *MockBrandingThemeAPI
}

// NewMockBrandingThemeAPI creates a new mock instance.
func NewMockBrandingThemeAPI(ctrl *gomock.Controller) *MockBrandingThemeAPI {
	mock := &MockBrandingThemeAPI{ctrl: ctrl}
	mock.recorder = &MockBrandingThemeAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBrandingThemeAPI) EXPECT() *MockBrandingThemeAPIMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockBrandingThemeAPI) Create(ctx context.Context, theme *management.BrandingTheme, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, theme}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockBrandingThemeAPIMockRecorder) Create(ctx, theme interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, theme}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockBrandingThemeAPI)(nil).Create), varargs...)
}

// Default mocks base method.
func (m *MockBrandingThemeAPI) Default(ctx context.Context, opts ...management.RequestOption) (*management.BrandingTheme, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Default", varargs...)
	ret0, _ := ret[0].(*management.BrandingTheme)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Default indicates an expected call of Default.
func (mr *MockBrandingThemeAPIMockRecorder) Default(ctx interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Default", reflect.TypeOf((*MockBrandingThemeAPI)(nil).Default), varargs...)
}

// Delete mocks base method.
func (m *MockBrandingThemeAPI) Delete(ctx context.Context, id string, opts ...management.RequestOption) error {
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
func (mr *MockBrandingThemeAPIMockRecorder) Delete(ctx, id interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockBrandingThemeAPI)(nil).Delete), varargs...)
}

// Read mocks base method.
func (m *MockBrandingThemeAPI) Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.BrandingTheme, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Read", varargs...)
	ret0, _ := ret[0].(*management.BrandingTheme)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockBrandingThemeAPIMockRecorder) Read(ctx, id interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockBrandingThemeAPI)(nil).Read), varargs...)
}

// Update mocks base method.
func (m *MockBrandingThemeAPI) Update(ctx context.Context, id string, theme *management.BrandingTheme, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, id, theme}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockBrandingThemeAPIMockRecorder) Update(ctx, id, theme interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, id, theme}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockBrandingThemeAPI)(nil).Update), varargs...)
}
