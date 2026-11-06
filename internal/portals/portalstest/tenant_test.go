package portalstest_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

func TestTenant_IDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()

	server, err := tenant.CreateResourceServer(ctx, portals.ResourceServerSpec{Name: "API", Identifier: "https://acme/me/"})
	require.NoError(t, err)
	client, err := tenant.CreateClient(ctx, portals.ClientSettings{Name: "App"})
	require.NoError(t, err)
	grant, err := tenant.CreateClientGrant(ctx, portals.ClientGrantSpec{ClientID: client.ID, Audience: server.Identifier})
	require.NoError(t, err)
	vault, err := tenant.CreateVaultConnection(ctx, "Vault", portals.VaultSetup{})
	require.NoError(t, err)
	form, err := tenant.ImportForm(ctx, map[string]any{"form": map[string]any{"name": "Profile"}})
	require.NoError(t, err)
	portal, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "acme", Client: &portals.PortalClient{ClientID: client.ID}})
	require.NoError(t, err)

	// One counter feeds every kind of ID, in the order the resources are created, with the shape
	// of the real IDs. Forms use two numbers: one for the form and one for its flow.
	assert.Equal(t, "000000000000000000000002", server.ID)
	assert.Equal(t, "00000000000000000000000000000003", client.ID)
	assert.Equal(t, "cgr_0000000000000004", grant.ID)
	assert.Equal(t, "ac_0000000000000000000005", vault.ID)
	assert.Equal(t, "ap_0000000000000000000006", form.ID)
	assert.Equal(t, "af_0000000000000000000007", form.FlowID)
	assert.Equal(t, "up_0000000000000000000008", portal.ID)
}

func TestTenant_Conflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		create func(ctx context.Context, tenant *portalstest.Tenant) error
	}{
		{
			name: "a resource server identifier already in use",
			create: func(ctx context.Context, tenant *portalstest.Tenant) error {
				_, err := tenant.CreateResourceServer(ctx, portals.ResourceServerSpec{Identifier: "https://acme/me/"})
				return err
			},
		},
		{
			name: "a portal slug already in use",
			create: func(ctx context.Context, tenant *portalstest.Tenant) error {
				_, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "acme", Client: &portals.PortalClient{}})
				return err
			},
		},
		{
			name: "an organization name already in use",
			create: func(ctx context.Context, tenant *portalstest.Tenant) error {
				_, err := tenant.CreateOrganization(ctx, portals.OrganizationSpec{Name: "acme"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			tenant := portalstest.New()
			tenant.AddResourceServer(portals.ResourceServer{ID: "rs_1", Identifier: "https://acme/me/"})
			tenant.AddPortal(portals.Portal{ID: "up_1", Slug: "acme"})
			tenant.AddOrganization(portals.Organization{ID: "org_1", Name: "acme"})

			err := tt.create(ctx, tenant)

			assert.True(t, portals.IsStatus(err, http.StatusConflict), "got %v", err)
		})
	}
}

func TestTenant_ClientSecrets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	created, err := tenant.CreateClient(ctx, portals.ClientSettings{Name: "App", Metadata: map[string]string{"role": "portal"}})
	require.NoError(t, err)

	listed, err := tenant.ListClients(ctx)
	require.NoError(t, err)
	read, err := tenant.ReadClient(ctx, created.ID)
	require.NoError(t, err)
	secret, err := tenant.ReadClientSecret(ctx, created.ID)
	require.NoError(t, err)

	assert.NotEmpty(t, created.Secret, "a create returns the secret")
	assert.Empty(t, listed[0].Secret)
	assert.Empty(t, read.Secret, "a read of the settings does not return the secret")
	assert.Equal(t, created.Secret, secret)
	assert.Equal(t, map[string]string{"role": "portal"}, listed[0].Metadata)
	assert.Nil(t, listed[0].Callbacks, "a list returns only the ID, name, metadata and session transfer")
}

func TestTenant_UpdateClientKeepsWhatIsNotSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	created, err := tenant.CreateClient(ctx, portals.ClientSettings{
		Name:              "App",
		Callbacks:         []string{"https://old/callback"},
		OrganizationUsage: "allow",
		SessionTransfer:   &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"cookie"}},
	})
	require.NoError(t, err)

	require.NoError(t, tenant.UpdateClient(ctx, created.ID, portals.ClientSettings{Callbacks: []string{"https://new/callback"}}))

	read, err := tenant.ReadClient(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://new/callback"}, read.Callbacks)
	assert.Equal(t, "App", read.Name)
	assert.Equal(t, "allow", read.OrganizationUsage)
	assert.Equal(t, []string{"cookie"}, read.SessionTransfer.AllowedAuthenticationMethods)
}

func TestTenant_ReadForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{name: "an ID the API cannot match is a validation error", id: "not-a-form-id", wantStatus: http.StatusBadRequest},
		{name: "a form that does not exist", id: "ap_0000000000000000000099", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := portalstest.New().ReadForm(context.Background(), tt.id)

			assert.True(t, portals.IsStatus(err, tt.wantStatus), "got %v", err)
		})
	}
}

func TestTenant_VaultSessionCarriesTheScopesOfItsGrant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	client, err := tenant.CreateClient(ctx, portals.ClientSettings{Name: "Vault app"})
	require.NoError(t, err)
	vault, err := tenant.CreateVaultConnection(ctx, "Vault", portals.VaultSetup{ClientID: client.ID})
	require.NoError(t, err)
	_, err = tenant.CreateClientGrant(ctx, portals.ClientGrantSpec{
		ClientID: client.ID,
		Audience: "https://acme.us.auth0.com/api/v2/",
		Scope:    []string{"update:users"},
	})
	require.NoError(t, err)
	assert.Empty(t, tenant.VaultMintedScopes(vault.ID), "the session was minted before the grant existed")

	require.NoError(t, tenant.UpdateVaultConnection(ctx, vault.ID, portals.VaultSetup{ClientID: client.ID}))

	assert.Equal(t, []string{"update:users"}, tenant.VaultMintedScopes(vault.ID))
}

func TestTenant_Probe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{name: "a read succeeds", method: http.MethodGet, path: "clients?per_page=1"},
		{name: "a create with an empty body is a validation error", method: http.MethodPost, path: "clients", body: map[string]any{}, wantStatus: http.StatusBadRequest},
		{name: "an update of a resource that does not exist", method: http.MethodPatch, path: "client-grants/cgr_preflight0000000", body: map[string]any{}, wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := portalstest.New().Probe(context.Background(), tt.method, tt.path, tt.body)

			if tt.wantStatus == 0 {
				assert.NoError(t, err)
				return
			}
			assert.True(t, portals.IsStatus(err, tt.wantStatus), "got %v", err)
		})
	}
}

func TestTenant_DenyAndFailNext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	tenant.Deny("POST clients", "Insufficient scope, expected any of: create:clients")
	tenant.FailNext("POST portals", http.StatusInternalServerError)

	_, deniedErr := tenant.CreateClient(ctx, portals.ClientSettings{Name: "App"})
	probeErr := tenant.Probe(ctx, http.MethodPost, "clients", map[string]any{})
	_, firstErr := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "a", Client: &portals.PortalClient{}})
	_, secondErr := tenant.CreatePortal(ctx, portals.CreatePortalRequest{Slug: "a", Client: &portals.PortalClient{}})

	assert.Equal(t, "insufficient_scope", portals.CodeOf(deniedErr))
	assert.True(t, portals.IsStatus(deniedErr, http.StatusForbidden))
	assert.True(t, portals.IsStatus(probeErr, http.StatusForbidden), "a probe is denied like the write it stands for")
	assert.True(t, portals.IsStatus(firstErr, http.StatusInternalServerError))
	assert.NoError(t, secondErr, "a failure happens once")
	assert.Equal(t, []string{"POST clients", "POST clients", "POST portals", "POST portals"}, tenant.Calls())
	assert.Equal(t, tenant.Calls(), tenant.Writes())
}

func TestTenant_ReturnsCopies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	created, err := tenant.CreatePortal(ctx, portals.CreatePortalRequest{
		Slug:   "acme",
		Client: &portals.PortalClient{ClientID: "client"},
		Pages:  &portals.Pages{Default: "profile"},
	})
	require.NoError(t, err)

	created.Pages.Default = "changed by the caller"

	read, err := tenant.ReadPortal(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "profile", read.Pages.Default)
}

func TestPrompter(t *testing.T) {
	t.Parallel()

	t.Run("a question nobody answers fails the test", func(t *testing.T) {
		t.Parallel()

		prompter := &portalstest.Prompter{}

		_, askErr := prompter.Ask("Name", "", "")
		_, confirmErr := prompter.Confirm("Sure?", true)
		_, chooseErr := prompter.Choose("Which?", nil)
		_, checklistErr := prompter.Checklist("Which ones?", nil)

		for _, err := range []error{askErr, confirmErr, chooseErr, checklistErr} {
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unexpected question")
		}

		assert.Equal(t, []string{"Name", "Sure?", "Which?", "Which ones?"}, prompter.Questions())
	})

	t.Run("a question with an answer gets it", func(t *testing.T) {
		t.Parallel()

		prompter := &portalstest.Prompter{
			AskFunc: func(_, defaultValue, _ string) (string, error) { return defaultValue + "!", nil },
		}

		answer, err := prompter.Ask("Name", "Acme", "")

		require.NoError(t, err)
		assert.Equal(t, "Acme!", answer)
	})
}

func TestReporter(t *testing.T) {
	t.Parallel()

	reporter := &portalstest.Reporter{}

	reporter.Section("Forms")
	reporter.Item("Profile", "created")
	reporter.Warn("The slug is taken.")
	require.NoError(t, reporter.Task("Creating portal", func() error { return nil }))

	assert.Equal(t, []string{
		"section: Forms",
		"item: Profile | created",
		"warn: The slug is taken.",
		"task: Creating portal",
	}, reporter.Lines())
	assert.True(t, reporter.Contains("warn", "slug"))
	assert.False(t, reporter.Contains("error", "slug"))
}
