package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	poppyTestAgentURL = "https://muse.example/agent.json"
	poppyTestAPI      = "https://api.shop.example"
	poppyTestOtherAPI = "https://other.example/api"
)

func testPage[C comparable, T, R any](items ...T) *core.Page[C, T, R] {
	return &core.Page[C, T, R]{
		Results:      items,
		NextPageFunc: func(context.Context) (*core.Page[C, T, R], error) { return nil, core.ErrNoPages },
	}
}

func testServerPage(items ...*managementv3.ResourceServer) *auth0.ResourceServerPage {
	return testPage[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent](items...)
}

func testClientPage(items ...*managementv3.Client) *auth0.ClientListPage {
	return testPage[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent](items...)
}

func testPtr[T any](value T) *T { return &value }

func decodeCLIError(t *testing.T, err error) (reason string, details map[string]any) {
	t.Helper()

	require.Error(t, err)
	reason = errorReason(err)

	if raw := errorDetails(err); raw != nil {
		require.NoError(t, json.Unmarshal(raw, &details))
	}

	return reason, details
}

func TestPoppyError(t *testing.T) {
	forbidden := func(body string) error { return core.NewAPIError(http.StatusForbidden, nil, errors.New(body)) }

	t.Run("insufficient scope", func(t *testing.T) {
		c := &cli{}

		err := c.poppyError(forbidden(`{"errorCode":"insufficient_scope","message":"Insufficient scope, expected any of: read:agents"}`))

		assert.Equal(t, "insufficient_scope", errorReason(err))
		assert.Contains(t, err.Error(), "auth0 login --scopes read:agents")
	})

	t.Run("another 403", func(t *testing.T) {
		c := &cli{}

		err := c.poppyError(forbidden(`{"errorCode":"poppy_beta_off"}`))

		assert.EqualError(t, err, "the tenant may lack Agent as Principal or the Poppy beta; contact support")
	})

	t.Run("another 403 with debug", func(t *testing.T) {
		c := &cli{debug: true}

		err := c.poppyError(forbidden(`{"errorCode":"poppy_beta_off"}`))

		assert.EqualError(t, err, "the tenant may lack Agent as Principal or the Poppy beta; contact support (status 403)")
		assert.ErrorAs(t, err, new(*core.APIError))
	})

	t.Run("a partial failure is rewritten inside", func(t *testing.T) {
		c := &cli{}
		partial := &setup.PartialError{Applied: []string{"one"}, Remaining: []string{"two"}, Err: forbidden(`{"errorCode":"poppy_beta_off"}`)}

		err := c.poppyError(partial)

		assert.Same(t, partial, err)
		assert.ErrorContains(t, err, "the tenant may lack Agent as Principal")
	})

	t.Run("other errors pass through", func(t *testing.T) {
		c := &cli{}
		boom := errors.New("boom")

		assert.Same(t, boom, c.poppyError(boom))

		notFound := core.NewAPIError(http.StatusNotFound, nil, errors.New("missing"))
		assert.Same(t, notFound, c.poppyError(notFound))
	})
}
