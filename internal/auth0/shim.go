package auth0

import "gopkg.in/auth0.v5/management"

func New(m *management.Management) API {
	return &shim{m}
}

// shim provides a way to mock management.Management since it uses fields, and
// that's not possible to mock.
type shim struct {
	m *management.Management
}

func (s *shim) Actions() ActionsAPI {
	return s.m.Action
}

func (s *shim) Client() ClientAPI {
	return s.m.Client
}

func (s *shim) Connection() ConnectionAPI {
	return s.m.Connection
}

func (s *shim) Log() LogAPI {
	return s.m.Log
}

func (s *shim) ResourceServer() ResourceServerAPI {
	return s.m.ResourceServer
}

func (s *shim) Rule() RuleAPI {
	return s.m.Rule
}
