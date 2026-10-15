package cli

import (
	"context"
	"encoding/json"
	"io"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

// jsonWriteStub records the request runJSONWrite sends and replies with response,
// or with err when set, so a --data test can assert the request and the outcome.
type jsonWriteStub struct {
	rawHTTPClientStub
	response string
	err      error

	gotMethod string
	gotURI    string
	gotBody   string
}

func (s *jsonWriteStub) Request(_ context.Context, method, uri string, payload interface{}, _ ...management.RequestOption) error {
	s.gotMethod = method
	s.gotURI = uri
	if raw, ok := payload.(*json.RawMessage); ok {
		s.gotBody = string(*raw)
		if s.response != "" {
			*raw = json.RawMessage(s.response)
		}
	}
	return s.err
}

func newJSONWriteCLI(stub *jsonWriteStub) *cli {
	return &cli{
		api:      &auth0.API{HTTPClient: stub},
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
	}
}
