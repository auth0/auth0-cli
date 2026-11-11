package portals_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

const tenantDomain = "acme.us.auth0.com"

func consumerOptions() portals.SetupOptions {
	return portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme Account", Slug: "acme-account"}
}

// newSetupDeps returns what setup needs to run on a tenant without a terminal: any question fails
// the test, unless the test says how to answer it.
func newSetupDeps(tenant *portalstest.Tenant) (portals.SetupDeps, *portalstest.Reporter, *portalstest.Prompter) {
	reporter := &portalstest.Reporter{}
	prompter := &portalstest.Prompter{}

	return portals.SetupDeps{
		API:          tenant,
		Reporter:     reporter,
		Prompter:     prompter,
		TenantDomain: tenantDomain,
	}, reporter, prompter
}

// clientNamed returns the application whose name starts with the prefix.
func clientNamed(t *testing.T, tenant *portalstest.Tenant, prefix string) portals.Client {
	t.Helper()

	for _, client := range tenant.Clients() {
		if strings.HasPrefix(client.Name, prefix) {
			return client
		}
	}

	require.FailNow(t, "no application starts with "+prefix)

	return portals.Client{}
}

func runSetup(t *testing.T, tenant *portalstest.Tenant, options portals.SetupOptions) portals.SetupResult {
	t.Helper()

	deps, _, _ := newSetupDeps(tenant)
	result, err := portals.Setup(context.Background(), options, deps)
	require.NoError(t, err)

	return result
}

func TestSetup_ConsumerPortalEndToEnd(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	deps, reporter, _ := newSetupDeps(tenant)

	result, err := portals.Setup(context.Background(), consumerOptions(), deps)

	require.NoError(t, err)
	assert.Equal(t, portals.StatusCreated, result.Portal.Status)
	assert.Equal(t, "https://acme.us.auth0.com/portals/acme-account", result.Portal.URL)
	assert.Len(t, tenant.Portals(), 1)
	assert.Len(t, tenant.Forms(), 3)
	assert.Len(t, tenant.VaultConnections(), 1)

	secret := clientNamed(t, tenant, "Acme Account").Secret
	require.NotEmpty(t, secret)

	for _, line := range reporter.Lines() {
		assert.NotContains(t, line, secret, "the client secret must never be shown")
	}
}

func TestSetup_TheResultIsTheJSONTheToolPrinted(t *testing.T) {
	t.Parallel()

	result := runSetup(t, portalstest.New(), consumerOptions())

	got, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/consumer-setup.json")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

const customCodeAndPasskeysCost = "Custom code does not run on the domain of the tenant, " +
	"and passkeys enrolled there have to be enrolled again if the portal moves to a custom domain later."

func TestSetup_ShowsTheStepsInOrder(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	deps, reporter, _ := newSetupDeps(tenant)

	_, err := portals.Setup(context.Background(), consumerOptions(), deps)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"section: Permissions",
		"task: Checking permissions",
		"result: The CLI session has every permission setup needs",
		"warn: The tenant has no custom domain, so the portal is served from acme.us.auth0.com. " + customCodeAndPasskeysCost,
		"section: Resource servers",
		"task: Checking Auth0 My Account API",
		`item: "Auth0 My Account API" | created`,
		"result: Resource servers ready",
		"section: Application",
		"task: Creating application",
		`item: "Acme Account (Universal Portals)" | created`,
		"reference: Application | 00000000000000000000000000000003 | https://manage.auth0.com/dashboard/us/acme/applications/00000000000000000000000000000003/settings",
		"task: Granting My Account API",
		`item: "My Account API" access | created`,
		"result: Application ready",
		"section: Forms",
		"task: Preparing the forms vault",
		`item: "Universal Portals forms vault" | created`,
		"reference: Vault connection | ac_0000000000000000000007 | https://forms.auth0.com/tenants/us/acme/vault/apps/AUTH0/edit",
		"task: Creating forms",
		`item: "Profile update (Universal Portals)" | created`,
		"reference: Form | ap_0000000000000000000008 | https://forms.auth0.com/tenants/us/acme/forms/ap_0000000000000000000008/edit",
		"reference: Flow | af_0000000000000000000009 | https://forms.auth0.com/tenants/us/acme/flows/af_0000000000000000000009/edit",
		`item: "Marketing communication preferences (Universal Portals)" | created`,
		"reference: Form | ap_0000000000000000000010 | https://forms.auth0.com/tenants/us/acme/forms/ap_0000000000000000000010/edit",
		"reference: Flow | af_0000000000000000000011 | https://forms.auth0.com/tenants/us/acme/flows/af_0000000000000000000011/edit",
		`item: "Privacy settings (Universal Portals)" | created`,
		"reference: Form | ap_0000000000000000000012 | https://forms.auth0.com/tenants/us/acme/forms/ap_0000000000000000000012/edit",
		"reference: Flow | af_0000000000000000000013 | https://forms.auth0.com/tenants/us/acme/flows/af_0000000000000000000013/edit",
		"result: Forms ready",
		"section: Portal",
		"task: Creating portal",
		`item: "Acme Account" | created`,
		"result: Portal ready",
		"reference: Portal | up_0000000000000000000014 | https://manage.auth0.com/dashboard/us/acme/universal-portals/up_0000000000000000000014/edit",
	}, reporter.Lines())
}

func TestSetup_TheApplicationOfAConsumerPortal(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	runSetup(t, tenant, consumerOptions())

	client := clientNamed(t, tenant, "Acme Account")

	assert.NotContains(t, client.GrantTypes, "client_credentials", "the portal application must not have client_credentials")
	assert.Equal(t, "non-rotating", client.RefreshToken.RotationType)
	assert.Equal(t, 3600, *client.RefreshToken.IdleTokenLifetime)
	assert.Equal(t, 86400, *client.RefreshToken.TokenLifetime)
	// The API keeps a lifetime infinite unless it is told otherwise, and then ignores the seconds.
	require.NotNil(t, client.RefreshToken.InfiniteTokenLifetime)
	assert.False(t, *client.RefreshToken.InfiniteTokenLifetime)
	require.NotNil(t, client.RefreshToken.InfiniteIdleTokenLifetime)
	assert.False(t, *client.RefreshToken.InfiniteIdleTokenLifetime)

	audiences := make([]string, 0)
	for _, policy := range client.RefreshToken.Policies {
		audiences = append(audiences, policy.Audience)
	}

	assert.Equal(t, []string{"https://acme.us.auth0.com/me/"}, audiences, "only the My Account policy")
	assert.Equal(t, map[string]string{"portals_setup_slug": "acme-account"}, client.Metadata)
	assert.Equal(t, []string{"query"}, client.SessionTransfer.AllowedAuthenticationMethods)
	assert.Equal(t, "ip", client.SessionTransfer.EnforceDeviceBinding)
}

func TestSetup_ARerunReusesEveryResourceAndCreatesNothing(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	runSetup(t, tenant, consumerOptions())

	counts := func() []int {
		return []int{len(tenant.Clients()), len(tenant.Grants()), len(tenant.Forms()), len(tenant.Portals()), len(tenant.VaultConnections())}
	}
	before := counts()

	rerun := runSetup(t, tenant, consumerOptions())

	assert.Equal(t, portals.StatusReused, rerun.Portal.Status)
	assert.Equal(t, portals.StatusReused, rerun.Application.Status)
	assert.Equal(t, portals.StatusReused, rerun.Vault.Status)
	assert.Equal(t, portals.StatusReused, rerun.Forms.PersonalInfo.Status)
	assert.Equal(t, portals.StatusReused, rerun.Forms.CommunicationPreferences.Status)
	assert.Equal(t, portals.StatusReused, rerun.Forms.PrivacyConsent.Status)
	assert.Equal(t, before, counts())
}

func TestSetup_AFailurePartwayIsRecoveredByRerunningTheSameCommand(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.FailNext("POST forms/import", http.StatusInternalServerError)
	deps, _, _ := newSetupDeps(tenant)

	_, err := portals.Setup(context.Background(), consumerOptions(), deps)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated failure")
	assert.Empty(t, tenant.Portals(), "the portal is the last step, so it must not exist yet")

	rerun := runSetup(t, tenant, consumerOptions())

	assert.Equal(t, portals.StatusCreated, rerun.Portal.Status)
	assert.Equal(t, portals.StatusReused, rerun.Application.Status)
	assert.Len(t, tenant.Clients(), 2, "one portal application and one forms vault application")
}

func TestSetup_ATakenSlug(t *testing.T) {
	t.Parallel()

	takenByAnother := func(tenant *portalstest.Tenant) {
		tenant.AddPortal(portals.Portal{ID: "up_other", Slug: "acme-account", Name: "Other", Client: &portals.PortalClient{ClientID: "cli_x"}})
	}

	t.Run("fails without a prompt when there is no terminal, before anything is created", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		takenByAnother(tenant)
		deps, _, _ := newSetupDeps(tenant)

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var taken *portals.SlugTakenError
		require.ErrorAs(t, err, &taken)
		assert.Equal(t, "acme-account", taken.Slug)
		assert.Contains(t, err.Error(), "already used by another portal")
		assert.Empty(t, tenant.Clients())
		assert.Empty(t, tenant.Grants())
		assert.Empty(t, tenant.VaultConnections())
	})

	t.Run("fails when the portal belongs to an application setup did not create", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		runSetup(t, tenant, consumerOptions())

		for _, client := range tenant.Clients() {
			tenant.DeleteClient(client.ID)
		}

		deps, _, _ := newSetupDeps(tenant)

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var taken *portals.SlugTakenError
		require.ErrorAs(t, err, &taken)
		assert.Equal(t, "acme-account", taken.Slug)
	})

	t.Run("is asked again when there is a terminal", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		takenByAnother(tenant)
		deps, reporter, prompter := newSetupDeps(tenant)
		deps.Interactive = true
		prompter.AskFunc = func(string, string, string) (string, error) { return "acme-account-2", nil }

		result, err := portals.Setup(context.Background(), consumerOptions(), deps)

		require.NoError(t, err)
		assert.Equal(t, "acme-account-2", result.Portal.Slug)
		assert.True(t, reporter.Contains("warn", `The slug "acme-account" is already taken.`))
	})

	t.Run("is asked again until the slug is valid, with the next free one as the suggestion", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		takenByAnother(tenant)
		deps, reporter, prompter := newSetupDeps(tenant)
		deps.Interactive = true

		answers := []string{"Not Valid", "acme-account-3"}
		var suggestions []string

		prompter.AskFunc = func(_, defaultValue, _ string) (string, error) {
			suggestions = append(suggestions, defaultValue)
			answer := answers[0]
			answers = answers[1:]

			return answer, nil
		}

		result, err := portals.Setup(context.Background(), consumerOptions(), deps)

		require.NoError(t, err)
		assert.Equal(t, "acme-account-3", result.Portal.Slug)
		assert.Equal(t, []string{"acme-account-2", "acme-account-2"}, suggestions)
		assert.True(t, reporter.Contains("error", "The slug must start with a lowercase letter"))
	})
}

func TestSetup_ARefusedPermissionStopsBeforeAnythingIsCreated(t *testing.T) {
	t.Parallel()

	t.Run("missing scopes name what to grant", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.Deny("POST portals", "Insufficient scope, expected any of: create:portals")
		tenant.Deny(
			"POST forms/import",
			"Insufficient scope, expected all of: create:forms,create:flows,create:flows_vault_connections,read:flows_vault_connections",
		)
		deps, _, _ := newSetupDeps(tenant)

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var missing *portals.MissingScopesError
		require.ErrorAs(t, err, &missing)
		assert.Equal(t, []string{
			"create:flows", "create:flows_vault_connections", "create:forms", "create:portals", "read:flows_vault_connections",
		}, missing.Scopes)
		assert.Empty(t, tenant.Clients(), "the checks run before any create")
	})

	t.Run("the scopes are listed once, by the error, with one login command", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.Deny("POST portals", "Insufficient scope, expected any of: create:portals")
		deps, reporter, _ := newSetupDeps(tenant)

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var missing *portals.MissingScopesError
		require.ErrorAs(t, err, &missing)
		assert.Equal(t, "The CLI session is missing 1 permission(s) setup needs: create:portals. Nothing was created", err.Error())

		logins := 0

		for _, fix := range portals.FixesOf(err) {
			if strings.Contains(fix, "auth0 login --scopes") {
				logins++
			}
		}

		assert.Equal(t, 1, logins)

		for _, line := range reporter.Lines() {
			assert.NotContains(t, line, "create:portals", "the scopes are not shown before the error reports them")
		}
	})

	t.Run("a 403 that names no scope is not reported as a pass", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		tenant.FailNext("GET portals", http.StatusForbidden)
		deps, reporter, _ := newSetupDeps(tenant)

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var refused *portals.PermissionCheckError
		require.ErrorAs(t, err, &refused)
		assert.Contains(t, err.Error(), `"list portals" with HTTP 403`)
		assert.Contains(t, err.Error(), "not enabled for acme.us.auth0.com")
		assert.Empty(t, tenant.Clients())
		assert.Empty(t, tenant.ResourceServers())
		assert.False(t, reporter.Contains("result", "every permission"))
	})

	t.Run("a feature that is not enabled", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, _, _ := newSetupDeps(tenant)
		deps.Prober = failingProber{err: &portals.APIError{Status: 403, Code: "feature_not_enabled", Message: "Feature not enabled"}}

		_, err := portals.Setup(context.Background(), consumerOptions(), deps)

		var notEnabled *portals.FeatureNotEnabledError
		require.ErrorAs(t, err, &notEnabled)
		assert.Empty(t, tenant.Clients())
	})
}

// failingProber fails every check with the same error.
type failingProber struct {
	err error
}

func (p failingProber) Probe(context.Context, string, string, any) error { return p.err }

func TestSetup_ACheckThatCouldNotBeVerifiedIsAWarningNotAPass(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.FailNext("GET portals", http.StatusNotFound)
	deps, reporter, _ := newSetupDeps(tenant)

	_, err := portals.Setup(context.Background(), consumerOptions(), deps)

	require.NoError(t, err)
	assert.True(t, reporter.Contains("warn", `Could not verify "list portals"`))
	assert.True(t, reporter.Contains("result", "No permission is missing, but the checks above could not be verified"))
	assert.False(t, reporter.Contains("result", "every permission"))
}

func TestSetup_ACustomDomainIsUsedForTheURLAndTheBackchannelLogout(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.SetCustomDomain(&portals.CustomDomain{Domain: "auth.acme.com", Status: "ready"})

	result := runSetup(t, tenant, consumerOptions())

	assert.Equal(t, "https://auth.acme.com/portals/acme-account", result.Portal.URL)

	client := clientNamed(t, tenant, "Acme Account")
	assert.Equal(t, []string{"https://auth.acme.com/portals/auth/callback"}, client.Callbacks)
	assert.Equal(t, []string{"https://auth.acme.com"}, client.AllowedLogoutURLs)
	require.NotNil(t, client.OIDCLogout, "back-channel logout is set on a custom domain")
	assert.Equal(t, []string{"https://auth.acme.com/portals/auth/backchannel-logout"}, client.OIDCLogout.BackchannelLogoutURLs)
	assert.Equal(t, "custom", client.OIDCLogout.Mode)
}

func TestSetup_ACustomDomainThatIsNotReadyIsNotUsed(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.SetCustomDomain(&portals.CustomDomain{Domain: "auth.acme.com", Status: "pending_verification"})

	result := runSetup(t, tenant, consumerOptions())

	assert.Equal(t, "https://acme.us.auth0.com/portals/acme-account", result.Portal.URL)
	assert.Nil(t, clientNamed(t, tenant, "Acme Account").OIDCLogout)
}

func TestSetup_TheCustomDomainThePortalIsServedFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		domains     []portals.CustomDomain
		wantHost    string
		wantWarning string
	}{
		{
			name:     "a ready primary domain is used when none is the default",
			domains:  []portals.CustomDomain{{Domain: "bad.acme.com", Status: "failed"}, {Domain: "auth.acme.com", Status: "ready", Primary: true}},
			wantHost: "auth.acme.com",
		},
		{
			name:     "the only ready domain is used when none is the default or the primary",
			domains:  []portals.CustomDomain{{Domain: "bad.acme.com", Status: "failed"}, {Domain: "auth.acme.com", Status: "ready"}},
			wantHost: "auth.acme.com",
		},
		{
			name:        "several ready domains with no default or primary are not guessed",
			domains:     []portals.CustomDomain{{Domain: "a.acme.com", Status: "ready"}, {Domain: "b.acme.com", Status: "ready"}},
			wantHost:    tenantDomain,
			wantWarning: "Several custom domains are ready",
		},
		{
			name:        "a domain that is not ready is not used, and the tenant is told what that costs",
			domains:     []portals.CustomDomain{{Domain: "bad.acme.com", Status: "pending_verification"}},
			wantHost:    tenantDomain,
			wantWarning: "No custom domain is ready",
		},
		{
			name:        "a tenant with no custom domain is told what that costs",
			wantHost:    tenantDomain,
			wantWarning: "passkeys",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tenant := portalstest.New()
			for _, domain := range test.domains {
				tenant.AddCustomDomain(domain)
			}

			deps, reporter, _ := newSetupDeps(tenant)
			result, err := portals.Setup(context.Background(), consumerOptions(), deps)
			require.NoError(t, err)

			assert.Equal(t, "https://"+test.wantHost+"/portals/acme-account", result.Portal.URL)
			assert.Equal(t, []string{"https://" + test.wantHost + "/portals/auth/callback"}, clientNamed(t, tenant, "Acme Account").Callbacks)

			if test.wantWarning == "" {
				assert.False(t, reporter.Contains("warn", "custom domain"), "nothing to warn about")
			} else {
				assert.True(t, reporter.Contains("warn", test.wantWarning), "the warning should say: %s", test.wantWarning)
			}
		})
	}
}

func TestSetup_TheDefaultCustomDomainWinsOverThePrimary(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.AddCustomDomain(portals.CustomDomain{Domain: "primary.acme.com", Status: "ready", Primary: true})
	tenant.SetCustomDomain(&portals.CustomDomain{Domain: "default.acme.com", Status: "ready"})

	result := runSetup(t, tenant, consumerOptions())

	assert.Equal(t, "https://default.acme.com/portals/acme-account", result.Portal.URL)
}

func TestSetup_TheFormsVaultIsRefreshedWhenItsGrantGainsAScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenant := portalstest.New()
	runSetup(t, tenant, consumerOptions())

	vaultApp := clientNamed(t, tenant, portals.FormsVaultName())

	for _, grant := range tenant.Grants() {
		if grant.ClientID == vaultApp.ID {
			require.NoError(t, tenant.UpdateClientGrantScope(ctx, grant.ID, []string{"update:users"}))
		}
	}

	deps, reporter, _ := newSetupDeps(tenant)

	rerun, err := portals.Setup(ctx, consumerOptions(), deps)

	require.NoError(t, err)
	assert.Equal(t, portals.StatusReused, rerun.Vault.Status)
	assert.Equal(t, portals.FormsVaultScopes(), tenant.VaultMintedScopes(rerun.Vault.ID))
	assert.True(t, reporter.Contains("item", "Forms vault refreshed with the updated permissions"))
}

// What a person can get wrong in the options is told before anything is sent, even before the check
// of the permissions: a session that lacks a scope should not have to fix it only to be told about a
// typo in the slug.
func TestSetup_RejectsWhatItCannotRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options portals.SetupOptions
		want    string
		wantErr any
	}{
		{
			name:    "a slug the API would refuse",
			options: portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme", Slug: "Not Valid"},
			want:    `The slug "Not Valid" must match ^[a-z][a-z0-9-]{0,99}$`,
			wantErr: new(*portals.UsageError),
		},
		{
			name:    "a template nobody knows",
			options: portals.SetupOptions{Template: "enterprise", Name: "Acme", Slug: "acme"},
			want:    `unknown template "enterprise": use consumer or business`,
			wantErr: new(*portals.UsageError),
		},
		{
			name:    "a name that is blank",
			options: portals.SetupOptions{Template: portals.TemplateConsumer, Name: "  ", Slug: "acme"},
			want:    "A portal needs a name",
			wantErr: new(*portals.UsageError),
		},
		{
			name:    "a business portal with nobody to ask for the user",
			options: portals.SetupOptions{Template: portals.TemplateBusiness, Name: "Acme", Slug: "acme"},
			want:    "A Business portal needs the user who will test it",
			wantErr: new(*portals.BusinessAccessError),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tenant := portalstest.New()
			deps, _, _ := newSetupDeps(tenant)

			_, err := portals.Setup(context.Background(), tt.options, deps)

			require.ErrorAs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, err.Error())
			assert.Empty(t, tenant.Calls(), "nothing is sent, not even a check of the permissions")
		})
	}
}

func TestSetup_ANameThatIsInUseByAnotherApplicationIsNeverReused(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.AddClient(portals.Client{ID: "cli_manual", Secret: "x", ClientSettings: portals.ClientSettings{Name: "Acme Account (Universal Portals)"}})
	deps, _, _ := newSetupDeps(tenant)

	_, err := portals.Setup(context.Background(), consumerOptions(), deps)

	var inUse *portals.NameInUseError
	require.ErrorAs(t, err, &inUse)
	assert.Equal(t,
		`An application named "Acme Account (Universal Portals)" already exists and was not created by this setup. Rename or delete it, then rerun`,
		err.Error(),
	)
	assert.Empty(t, tenant.Portals())
}

func TestSetup_ASlugTakenWhileSetupRuns(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	tenant.FailNext("POST portals", http.StatusConflict)
	deps, _, _ := newSetupDeps(tenant)

	_, err := portals.Setup(context.Background(), consumerOptions(), deps)

	var race *portals.SlugRaceError
	require.ErrorAs(t, err, &race)
	assert.Equal(t, `The slug "acme-account" was taken while setup was running. Rerun with a different --name and --slug`, err.Error())
}

func TestAskForMissing(t *testing.T) {
	t.Parallel()

	answerWithDefaults := func(prompter *portalstest.Prompter) {
		prompter.AskFunc = func(_, defaultValue, _ string) (string, error) { return defaultValue, nil }
		prompter.ChooseFunc = func(string, []portals.Choice) (int, error) { return 1, nil }
	}

	t.Run("it asks for the kind, the name and the slug, in that order", func(t *testing.T) {
		t.Parallel()

		prompter := &portalstest.Prompter{}
		answerWithDefaults(prompter)

		options, err := portals.AskForMissing(portals.SetupOptions{}, prompter)

		require.NoError(t, err)
		assert.Equal(t, portals.SetupOptions{Template: portals.TemplateBusiness, Name: "My Portal", Slug: "my-portal"}, options)
		assert.Equal(t, []string{"Which kind of portal do you want to set up?", "Portal name", "URL slug"}, prompter.Questions())
	})

	t.Run("the slug defaults to the one made from the name", func(t *testing.T) {
		t.Parallel()

		prompter := &portalstest.Prompter{}
		answerWithDefaults(prompter)

		options, err := portals.AskForMissing(portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme Admin"}, prompter)

		require.NoError(t, err)
		assert.Equal(t, "acme-admin", options.Slug)
		assert.Equal(t, []string{"URL slug"}, prompter.Questions())
	})

	t.Run("it asks for nothing that was given", func(t *testing.T) {
		t.Parallel()

		given := portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme", Slug: "acme-portal", UserQuery: "jane@acme.com"}

		options, err := portals.AskForMissing(given, &portalstest.Prompter{})

		require.NoError(t, err)
		assert.Equal(t, given, options)
	})

	t.Run("a cancel at a question is passed on", func(t *testing.T) {
		t.Parallel()

		prompter := &portalstest.Prompter{AskFunc: func(string, string, string) (string, error) { return "", portals.ErrCancelled }}

		_, err := portals.AskForMissing(portals.SetupOptions{Template: portals.TemplateConsumer}, prompter)

		assert.ErrorIs(t, err, portals.ErrCancelled)
	})
}

func TestSetup_ASlugLeftEmptyIsMadeFromTheName(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	deps, _, _ := newSetupDeps(tenant)

	result, err := portals.Setup(context.Background(), portals.SetupOptions{Template: portals.TemplateConsumer, Name: "Acme Account"}, deps)

	require.NoError(t, err)
	assert.Equal(t, "acme-account", tenant.Portals()[0].Slug)
	assert.Equal(t, portals.StatusCreated, result.Portal.Status)
}
