package portals_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

var businessApplication = portals.PortalApplicationInput{
	Name:         "Acme Admin (Universal Portals)",
	Slug:         "acme-admin",
	Template:     portals.TemplateBusiness,
	TenantDomain: tenantDomain,
	Host:         tenantDomain,
}

func TestEnsureResourceServer(t *testing.T) {
	t.Parallel()

	spec := portals.ResourceServerSpec{Name: "My Account", Identifier: "https://acme.us.auth0.com/me/"}

	t.Run("it is created once and reused on rerun", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		tenant := portalstest.New()

		first, err := portals.EnsureResourceServer(ctx, tenant, spec)
		require.NoError(t, err)
		second, err := portals.EnsureResourceServer(ctx, tenant, spec)
		require.NoError(t, err)

		assert.Equal(t, portals.StatusCreated, first.Status)
		assert.Equal(t, portals.StatusReused, second.Status)
		assert.Equal(t, first.ID, second.ID)
		assert.Len(t, tenant.ResourceServers(), 1)
	})

	t.Run("one created by another run in the meantime is reused", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.AddResourceServer(portals.ResourceServer{ID: "rs_other_run", Identifier: spec.Identifier})

		// The first read does not see the resource server, which is how another run creates it between the read and the create.
		outcome, err := portals.EnsureResourceServer(context.Background(), &blindOnce{ResourceServerStore: tenant}, spec)

		require.NoError(t, err)
		assert.Equal(t, portals.StatusReused, outcome.Status)
		assert.Equal(t, "rs_other_run", outcome.ID)
	})
}

// blindOnce hides what a tenant has from the first list of resource servers.
type blindOnce struct {
	portals.ResourceServerStore
	listed bool
}

func (b *blindOnce) ListResourceServers(ctx context.Context) ([]portals.ResourceServer, error) {
	if !b.listed {
		b.listed = true
		return nil, nil
	}

	return b.ResourceServerStore.ListResourceServers(ctx)
}

func TestEnsurePortalClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("a rerun returns the same client and secret, without creating a second client", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()

		first, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)
		second, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)

		assert.Equal(t, portals.StatusCreated, first.Status)
		assert.Equal(t, portals.StatusReused, second.Status)
		assert.Equal(t, first.ClientID, second.ClientID)
		assert.Equal(t, first.ClientSecret, second.ClientSecret)
		assert.Len(t, tenant.Clients(), 1)
	})

	t.Run("a rerun turns on session transfer for an application that lacks it", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.AddClient(portals.Client{ID: "cli_1", Secret: "s", ClientSettings: portals.ClientSettings{
			Name:     businessApplication.Name,
			Metadata: map[string]string{"portals_setup_slug": businessApplication.Slug},
		}})

		_, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)

		read, err := tenant.ReadClient(ctx, "cli_1")
		require.NoError(t, err)
		assert.Equal(t, &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"query"}, EnforceDeviceBinding: "ip"}, read.SessionTransfer)
	})

	t.Run("a rerun keeps a session transfer setting that was changed on purpose", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		first, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)
		require.NoError(t, tenant.UpdateClient(ctx, first.ClientID, portals.ClientSettings{
			SessionTransfer: &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"query"}, EnforceDeviceBinding: "asn"},
		}))

		_, err = portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)

		read, err := tenant.ReadClient(ctx, first.ClientID)
		require.NoError(t, err)
		assert.Equal(t, "asn", read.SessionTransfer.EnforceDeviceBinding)
	})

	t.Run("a rerun brings back what drifted, and leaves the rest", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		first, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)
		require.NoError(t, tenant.UpdateClient(ctx, first.ClientID, portals.ClientSettings{
			Callbacks:  []string{"https://elsewhere/callback"},
			GrantTypes: []string{"authorization_code", "client_credentials"},
		}))

		_, err = portals.EnsurePortalClient(ctx, tenant, businessApplication)
		require.NoError(t, err)

		read, err := tenant.ReadClient(ctx, first.ClientID)
		require.NoError(t, err)
		assert.Equal(t, []string{"https://acme.us.auth0.com/portals/auth/callback"}, read.Callbacks)
		assert.NotContains(t, read.GrantTypes, "client_credentials")
		assert.Equal(t, "Acme Admin (Universal Portals)", read.Name)
	})

	t.Run("a same-named application that setup did not create is never reused", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.AddClient(portals.Client{ID: "cli_manual", Secret: "x", ClientSettings: portals.ClientSettings{Name: businessApplication.Name}})

		_, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)

		var inUse *portals.NameInUseError
		require.ErrorAs(t, err, &inUse)
		assert.Len(t, tenant.Clients(), 1)
	})

	t.Run("a same-named application made for another slug is never reused either", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.AddClient(portals.Client{ID: "cli_other", Secret: "x", ClientSettings: portals.ClientSettings{
			Name:     businessApplication.Name,
			Metadata: map[string]string{"portals_setup_slug": "another-slug"},
		}})

		_, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)

		var inUse *portals.NameInUseError
		require.ErrorAs(t, err, &inUse)
	})

	t.Run("an application whose secret the session cannot read asks for read:client_keys", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.AddClient(portals.Client{ID: "cli_1", Secret: "", ClientSettings: portals.ClientSettings{
			Name:     businessApplication.Name,
			Metadata: map[string]string{"portals_setup_slug": businessApplication.Slug},
		}})

		_, err := portals.EnsurePortalClient(ctx, tenant, businessApplication)

		var unreadable *portals.ClientSecretUnreadableError
		require.ErrorAs(t, err, &unreadable)
		assert.Contains(t, portals.FixesOf(err)[0], "read:client_keys")
	})
}

func TestPortalClientSettings(t *testing.T) {
	t.Parallel()

	t.Run("the application accepts session transfer tokens from native applications", func(t *testing.T) {
		t.Parallel()

		settings := portals.PortalClientSettings(businessApplication)

		assert.Equal(t, &portals.SessionTransfer{AllowedAuthenticationMethods: []string{"query"}, EnforceDeviceBinding: "ip"}, settings.SessionTransfer)
	})

	t.Run("the application is a first-party web application that signs in with a secret in the body", func(t *testing.T) {
		t.Parallel()

		settings := portals.PortalClientSettings(businessApplication)

		assert.Equal(t, "regular_web", settings.AppType)
		assert.True(t, *settings.IsFirstParty)
		assert.True(t, *settings.OIDCConformant)
		assert.Equal(t, "client_secret_post", settings.TokenEndpointAuthMethod)
		assert.Equal(t, "allow", settings.OrganizationUsage)
		assert.Equal(t, "no_prompt", settings.OrganizationRequireBehavior)
		assert.Equal(t, []string{"https://acme.us.auth0.com"}, settings.AllowedLogoutURLs)
		assert.Nil(t, settings.OIDCLogout, "back-channel logout is only set on a custom domain")
	})

	t.Run("a refresh token policy for each API the template calls, with its scopes", func(t *testing.T) {
		t.Parallel()

		settings := portals.PortalClientSettings(businessApplication)

		got := map[string][]string{}
		for _, policy := range settings.RefreshToken.Policies {
			got[policy.Audience] = policy.Scope
		}

		assert.Equal(t, portals.AudiencesFor(portals.TemplateBusiness, tenantDomain), got)
		assert.Equal(t, portals.MyOrganizationScopes(), got["https://acme.us.auth0.com/my-org/"])
		assert.Equal(t, portals.MyAccountScopes(), got["https://acme.us.auth0.com/me/"])
	})
}

func TestEnsureClientGrant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	audience := "https://acme.us.auth0.com/me/"

	created, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: []string{"read:me:factors"}})
	require.NoError(t, err)
	reused, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: []string{"read:me:factors"}})
	require.NoError(t, err)
	reordered, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: []string{"read:me:factors", "read:me:factors"}})
	require.NoError(t, err)
	updated, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{
		ClientID: "cli_1",
		Audience: audience,
		Scope:    []string{"read:me:factors", "read:me:authentication_methods"},
	})
	require.NoError(t, err)

	assert.Equal(t, portals.StatusCreated, created.Status)
	assert.Equal(t, portals.StatusReused, reused.Status)
	assert.Equal(t, portals.StatusReused, reordered.Status, "the order and the repeats of the scopes do not matter")
	assert.Equal(t, portals.StatusUpdated, updated.Status)
	assert.Len(t, tenant.Grants(), 1)
	assert.ElementsMatch(t, []string{"read:me:factors", "read:me:authentication_methods"}, tenant.Grants()[0].Scope)
}

func TestEnsureClientGrant_ASubjectTypeIsAGrantOfItsOwn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	audience := "https://acme.us.auth0.com/me/"
	scope := []string{"read:me:factors"}

	forUsers, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: scope, SubjectType: "user"})
	require.NoError(t, err)
	forClient, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: scope, SubjectType: "client"})
	require.NoError(t, err)
	again, err := portals.EnsureClientGrant(ctx, tenant, portals.ClientGrantSpec{ClientID: "cli_1", Audience: audience, Scope: scope, SubjectType: "client"})
	require.NoError(t, err)

	assert.Equal(t, portals.StatusCreated, forClient.Status, "a grant for users does not give the application access for itself")
	assert.NotEqual(t, forUsers.ID, forClient.ID)
	assert.Equal(t, portals.StatusReused, again.Status)
	assert.Equal(t, forClient.ID, again.ID)
	assert.Len(t, tenant.Grants(), 2)
}

func TestVaultConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	setup := portals.VaultSetup{Domain: tenantDomain, ClientID: "cli_vault", ClientSecret: "s"}

	created, err := portals.EnsureVaultConnection(ctx, tenant, "Universal Portals forms vault", setup)
	require.NoError(t, err)
	reused, err := portals.EnsureVaultConnection(ctx, tenant, "Universal Portals forms vault", setup)
	require.NoError(t, err)

	assert.Equal(t, portals.StatusCreated, created.Status)
	assert.Equal(t, portals.StatusReused, reused.Status)
	assert.Equal(t, created.ID, reused.ID)
	assert.Len(t, tenant.VaultConnections(), 1)

	require.NoError(t, portals.RefreshVaultConnection(ctx, tenant, created.ID, setup))
	assert.Contains(t, tenant.Writes(), "PATCH flows/vault/connections/"+created.ID, "a refresh sends the setup again")
}

func TestEnsureForms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("forms are imported once and reused by name", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()

		first, err := portals.EnsureForms(ctx, tenant, "ac_vault")
		require.NoError(t, err)
		second, err := portals.EnsureForms(ctx, tenant, "ac_vault")
		require.NoError(t, err)

		require.Len(t, first, 3)
		assert.Equal(t, []portals.FormKey{portals.FormPersonalInfo, portals.FormCommunicationPreferences, portals.FormPrivacyConsent}, formKeys(first))

		for index := range first {
			assert.Equal(t, portals.StatusCreated, first[index].Status)
			assert.Equal(t, portals.StatusReused, second[index].Status)
			assert.Equal(t, first[index].ID, second[index].ID)
		}

		assert.Len(t, tenant.Forms(), 3)
	})

	t.Run("each form reports the flow it runs, whether it was created or reused", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()

		first, err := portals.EnsureForms(ctx, tenant, "ac_vault")
		require.NoError(t, err)
		second, err := portals.EnsureForms(ctx, tenant, "ac_vault")
		require.NoError(t, err)

		assert.NotEmpty(t, first[0].FlowID, "a created form reports its flow")
		assert.Equal(t, first[0].FlowID, second[0].FlowID)
	})

	t.Run("a failure leaves the forms that were imported, for the rerun to reuse", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		_, err := portals.EnsureForms(ctx, tenant, "ac_vault")
		require.NoError(t, err)
		tenant.DeleteForm(tenant.Forms()[2].ID)
		tenant.FailNext("POST forms/import", http.StatusInternalServerError)

		_, err = portals.EnsureForms(ctx, tenant, "ac_vault")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated failure")
		assert.Len(t, tenant.Forms(), 2)
	})
}

func formKeys(forms []portals.EnsuredForm) []portals.FormKey {
	keys := make([]portals.FormKey, 0, len(forms))
	for _, form := range forms {
		keys = append(keys, form.Key)
	}

	return keys
}

func TestEnsurePortal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	forms, err := portals.EnsureForms(ctx, tenant, "ac_vault")
	require.NoError(t, err)

	template, err := portals.TemplateFor(portals.TemplateConsumer, portals.FormIDs{
		portals.FormPersonalInfo:             forms[0].ID,
		portals.FormCommunicationPreferences: forms[1].ID,
		portals.FormPrivacyConsent:           forms[2].ID,
	})
	require.NoError(t, err)

	created, err := portals.EnsurePortal(ctx, tenant, template, "acme-admin", "Acme Admin", "cli_1", "s")
	require.NoError(t, err)
	reused, err := portals.EnsurePortal(ctx, tenant, template, "acme-admin", "Acme Admin", "cli_1", "s")
	require.NoError(t, err)
	_, refused := portals.EnsurePortal(ctx, tenant, template, "acme-admin", "Acme Admin", "cli_other", "s")

	assert.Equal(t, portals.StatusCreated, created.Status)
	assert.Equal(t, portals.StatusReused, reused.Status)
	assert.Equal(t, created.ID, reused.ID)

	var taken *portals.SlugTakenError
	require.ErrorAs(t, refused, &taken)
	assert.Equal(t, `The portal slug "acme-admin" is already used by another portal`, refused.Error())
}

func TestPortalURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		organization string
		want         string
	}{
		{name: "a consumer portal has no organization", want: "https://acme.us.auth0.com/portals/acme"},
		{name: "a business portal opens in an organization", organization: "org_1", want: "https://acme.us.auth0.com/portals/acme?organization=org_1"},
		{name: "an ID is escaped", organization: "org a/b", want: "https://acme.us.auth0.com/portals/acme?organization=org%20a%2Fb"},
		{name: "the placeholder stays readable", organization: "<ORGANIZATION_ID>", want: "https://acme.us.auth0.com/portals/acme?organization=<ORGANIZATION_ID>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, portals.PortalURL(tenantDomain, "acme", tt.organization))
		})
	}
}
