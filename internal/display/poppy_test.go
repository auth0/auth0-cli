package display

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	poppyShopAPI   = "https://api.shop.example"
	poppyShopApp   = "W1x9aBcD2eFgH3iJ"
	poppyTenantURL = "tenant.us.auth0.com"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var (
	unicode80 = Layout{Width: 80, Unicode: true}
	ascii80   = Layout{Width: 80}
	pipe      = Layout{Unicode: true}
)

// assertGolden compares got with testdata/<name>.golden.txt, and rewrites the
// file when the tests run with -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden.txt")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

// poppyConfiguredState is a tenant that setup has already been run against, with
// the config file it wrote.
func poppyConfiguredState(t *testing.T) *setup.State {
	t.Helper()

	file, err := config.Read(filepath.Join("..", "poppy", "config", "testdata", "poppy.golden.json"))
	require.NoError(t, err)

	return &setup.State{
		Tenant: setup.Tenant{
			CIMD:                     true,
			IssParameter:             true,
			ResourceParameterProfile: "compatibility",
			DefaultAudience:          poppyShopAPI,
			FriendlyName:             "Shop",
			CompanyDomains:           []string{"https://shop.example"},
		},
		Domain:       poppyTenantURL,
		CustomDomain: "auth.shop.example",
		APIs: []setup.API{{
			ID:         "rs_shop",
			Identifier: poppyShopAPI,
			Name:       "Shop API",
			Scopes: []setup.Scope{
				{Name: "orders:read", Description: "See your orders and their status. Read only."},
				{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
				{Name: setup.BrowserSignInScope, Description: setup.BrowserSignInDescription},
			},
			AllowOfflineAccess: true,
			AgentSubjectClaims: "auth0-v1",
			Poppy:              true,
			GuestPolicy:        "require_client_grant",
		}},
		App:  setup.App{ClientID: poppyShopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
		File: file,
	}
}

func poppyConfiguredInputs() setup.Inputs {
	return setup.Inputs{
		CompanyDomain: "shop.example",
		APIs:          []string{poppyShopAPI},
		App:           "Shop Web",
		Out:           config.DefaultFileName,
	}
}

// poppyFreshState is a tenant that has never been set up for Poppy.
func poppyFreshState(t *testing.T) (*setup.State, setup.Inputs) {
	t.Helper()

	state := poppyConfiguredState(t)
	state.Tenant = setup.Tenant{ResourceParameterProfile: "audience", FriendlyName: "Shop"}
	state.CustomDomain = ""
	state.APIs[0] = setup.API{
		ID:         "rs_shop",
		Identifier: poppyShopAPI,
		Name:       "Shop API",
		Scopes: []setup.Scope{
			{Name: "orders:read", Description: "See your orders and their status. Read only."},
			{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
		},
		AgentSubjectClaims: "off",
	}
	state.App.BrowserJoin = false
	state.File = nil

	in := poppyConfiguredInputs()
	in.APIURLs = map[string]string{poppyShopAPI: poppyShopAPI + "/openapi.json"}

	return state, in
}

func TestPoppyChangeCount(t *testing.T) {
	assert.Equal(t, "1 change", PoppyChangeCount(1))
	assert.Equal(t, "3 changes", PoppyChangeCount(3))
}
