// Code generated by MockGen. DO NOT EDIT.
// Source: branding_prompt.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	management "github.com/auth0/go-auth0/management"
	gomock "github.com/golang/mock/gomock"
)

// MockPromptAPI is a mock of PromptAPI interface.
type MockPromptAPI struct {
	ctrl     *gomock.Controller
	recorder *MockPromptAPIMockRecorder
}

// MockPromptAPIMockRecorder is the mock recorder for MockPromptAPI.
type MockPromptAPIMockRecorder struct {
	mock *MockPromptAPI
}

// NewMockPromptAPI creates a new mock instance.
func NewMockPromptAPI(ctrl *gomock.Controller) *MockPromptAPI {
	mock := &MockPromptAPI{ctrl: ctrl}
	mock.recorder = &MockPromptAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPromptAPI) EXPECT() *MockPromptAPIMockRecorder {
	return m.recorder
}

// CustomText mocks base method.
func (m *MockPromptAPI) CustomText(ctx context.Context, p, l string, opts ...management.RequestOption) (map[string]interface{}, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, p, l}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CustomText", varargs...)
	ret0, _ := ret[0].(map[string]interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CustomText indicates an expected call of CustomText.
func (mr *MockPromptAPIMockRecorder) CustomText(ctx, p, l interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, p, l}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CustomText", reflect.TypeOf((*MockPromptAPI)(nil).CustomText), varargs...)
}

// GetPartials mocks base method.
func (m *MockPromptAPI) GetPartials(ctx context.Context, prompt management.PromptType, opts ...management.RequestOption) (*management.PromptScreenPartials, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, prompt}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetPartials", varargs...)
	ret0, _ := ret[0].(*management.PromptScreenPartials)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPartials indicates an expected call of GetPartials.
func (mr *MockPromptAPIMockRecorder) GetPartials(ctx, prompt interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, prompt}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPartials", reflect.TypeOf((*MockPromptAPI)(nil).GetPartials), varargs...)
}

// ListRendering mocks base method.
func (m *MockPromptAPI) ListRendering(ctx context.Context, opts ...management.RequestOption) (*management.PromptRenderingList, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListRendering", varargs...)
	ret0, _ := ret[0].(*management.PromptRenderingList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListRendering indicates an expected call of ListRendering.
func (mr *MockPromptAPIMockRecorder) ListRendering(ctx interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListRendering", reflect.TypeOf((*MockPromptAPI)(nil).ListRendering), varargs...)
}

// Read mocks base method.
func (m *MockPromptAPI) Read(ctx context.Context, opts ...management.RequestOption) (*management.Prompt, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Read", varargs...)
	ret0, _ := ret[0].(*management.Prompt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockPromptAPIMockRecorder) Read(ctx interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockPromptAPI)(nil).Read), varargs...)
}

// ReadRendering mocks base method.
func (m *MockPromptAPI) ReadRendering(ctx context.Context, prompt management.PromptType, screen management.ScreenName, opts ...management.RequestOption) (*management.PromptRendering, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, prompt, screen}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ReadRendering", varargs...)
	ret0, _ := ret[0].(*management.PromptRendering)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadRendering indicates an expected call of ReadRendering.
func (mr *MockPromptAPIMockRecorder) ReadRendering(ctx, prompt, screen interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, prompt, screen}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadRendering", reflect.TypeOf((*MockPromptAPI)(nil).ReadRendering), varargs...)
}

// SetCustomText mocks base method.
func (m *MockPromptAPI) SetCustomText(ctx context.Context, p, l string, b map[string]interface{}, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, p, l, b}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetCustomText", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCustomText indicates an expected call of SetCustomText.
func (mr *MockPromptAPIMockRecorder) SetCustomText(ctx, p, l, b interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, p, l, b}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCustomText", reflect.TypeOf((*MockPromptAPI)(nil).SetCustomText), varargs...)
}

// SetPartials mocks base method.
func (m *MockPromptAPI) SetPartials(ctx context.Context, prompt management.PromptType, c *management.PromptScreenPartials, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, prompt, c}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetPartials", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetPartials indicates an expected call of SetPartials.
func (mr *MockPromptAPIMockRecorder) SetPartials(ctx, prompt, c interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, prompt, c}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPartials", reflect.TypeOf((*MockPromptAPI)(nil).SetPartials), varargs...)
}

// Update mocks base method.
func (m *MockPromptAPI) Update(ctx context.Context, p *management.Prompt, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, p}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockPromptAPIMockRecorder) Update(ctx, p interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, p}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockPromptAPI)(nil).Update), varargs...)
}

// UpdateRendering mocks base method.
func (m *MockPromptAPI) UpdateRendering(ctx context.Context, prompt management.PromptType, screen management.ScreenName, c *management.PromptRendering, opts ...management.RequestOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, prompt, screen, c}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateRendering", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateRendering indicates an expected call of UpdateRendering.
func (mr *MockPromptAPIMockRecorder) UpdateRendering(ctx, prompt, screen, c interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, prompt, screen, c}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateRendering", reflect.TypeOf((*MockPromptAPI)(nil).UpdateRendering), varargs...)
}
