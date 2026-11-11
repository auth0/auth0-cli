package portals_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

func businessOptions(userQuery string) portals.SetupOptions {
	return portals.SetupOptions{
		Template:  portals.TemplateBusiness,
		Name:      "Acme Admin",
		Slug:      "acme-admin",
		UserQuery: userQuery,
	}
}

// answers are the replies of a scripted prompter, in the order the questions come.
type answers struct {
	asks     []string
	chooses  []string
	confirms []bool
}

// prompter answers from the script. A choice is the option whose label contains the text.
func (a answers) prompter() *portalstest.Prompter {
	asks, chooses, confirms := a.asks, a.chooses, a.confirms

	return &portalstest.Prompter{
		AskFunc: func(question, _, _ string) (string, error) {
			if len(asks) == 0 {
				return "", errors.New("unexpected question: " + question)
			}

			next := asks[0]
			asks = asks[1:]

			return next, nil
		},
		ConfirmFunc: func(question string, _ bool) (bool, error) {
			if len(confirms) == 0 {
				return false, errors.New("unexpected confirmation: " + question)
			}

			next := confirms[0]
			confirms = confirms[1:]

			return next, nil
		},
		ChooseFunc: func(question string, options []portals.Choice) (int, error) {
			if len(chooses) == 0 {
				return 0, errors.New("unexpected choice: " + question)
			}

			want := chooses[0]
			chooses = chooses[1:]

			for index, option := range options {
				if strings.Contains(option.Label, want) {
					return index, nil
				}
			}

			return 0, errors.New("no option matching " + want)
		},
	}
}

func seedUser(tenant *portalstest.Tenant, id, email string) {
	tenant.AddUser(portals.User{ID: id, Email: email, Name: strings.Split(email, "@")[0]})
}

func permissionsOf(scopes []string) []portals.Permission {
	permissions := make([]portals.Permission, 0, len(scopes))
	for _, scope := range scopes {
		permissions = append(permissions, portals.Permission{Name: scope})
	}

	return permissions
}

// seedOrganization adds an organization with a connection, and the users that belong to it.
func seedOrganization(tenant *portalstest.Tenant, id, name string, members ...string) {
	tenant.AddOrganization(portals.Organization{ID: id, Name: name}, members...)
	tenant.EnableOrganizationConnectionFor(id, "con_db")
}

func seedDatabase(tenant *portalstest.Tenant) {
	tenant.AddConnection(portals.Connection{ID: "con_db", Name: "Username-Password-Authentication"}, "auth0")
}

func seedAdminRole(tenant *portalstest.Tenant) {
	tenant.AddRole(portals.Role{ID: "rol_admin", Name: "Organization - Admin"}, permissionsOf(portals.MyOrganizationScopes())...)
}

// interactiveDeps are the dependencies of a run with somebody to ask.
func interactiveDeps(tenant *portalstest.Tenant, script answers) (portals.SetupDeps, *portalstest.Reporter, *portalstest.Prompter) {
	deps, reporter, _ := newSetupDeps(tenant)
	prompter := script.prompter()
	deps.Prompter = prompter
	deps.Interactive = true

	return deps, reporter, prompter
}

func setupBusiness(tenant *portalstest.Tenant, options portals.SetupOptions, deps portals.SetupDeps) (portals.SetupResult, error) {
	return portals.Setup(context.Background(), options, deps)
}

func TestSetup_BusinessPortal(t *testing.T) {
	t.Parallel()

	t.Run("it is created with the organization in its URL", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|user1", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme-corp", "auth0|user1")
		seedAdminRole(tenant)
		tenant.AssignRole("org_1", "auth0|user1", "rol_admin")
		deps, _, _ := newSetupDeps(tenant)

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "https://acme.us.auth0.com/portals/acme-admin?organization=org_1", result.Portal.URL)
		assert.Equal(t, &portals.OrganizationResult{ID: "org_1", Name: "acme-corp", HasRole: true}, result.Organization)

		client := clientNamed(t, tenant, "Acme Admin")
		policies := map[string][]string{}

		for _, policy := range client.RefreshToken.Policies {
			policies[policy.Audience] = policy.Scope
		}

		assert.Equal(t, portals.AudiencesFor(portals.TemplateBusiness, tenantDomain), policies, "a policy for both APIs")

		grants := map[string][]string{}
		for _, grant := range tenant.Grants() {
			grants[grant.Audience] = grant.Scope
		}

		assert.Equal(t, portals.MyOrganizationScopes(), grants["https://acme.us.auth0.com/my-org/"])
		assert.Equal(t, portals.MyAccountScopes(), grants["https://acme.us.auth0.com/me/"])
		assert.Len(t, tenant.ResourceServers(), 2, "the My Organization API and the My Account API")
	})

	t.Run("it needs the user up front when nobody can be asked", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions(""), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, []string{"Rerun with --user <email or user id>."}, portals.FixesOf(err))
		assert.Empty(t, tenant.Clients())
	})

	t.Run("an unknown test user stops before the application, grants or vault are created", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("nobody@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, `No user matches "nobody@acme.com"`, err.Error())
		assert.Empty(t, tenant.Clients())
		assert.Empty(t, tenant.Grants())
		assert.Empty(t, tenant.VaultConnections())
		assert.Empty(t, tenant.Portals())
	})

	t.Run("a cancel at the test user question stops before anything is created", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, _, prompter := interactiveDeps(tenant, answers{})
		prompter.AskFunc = func(string, string, string) (string, error) { return "", portals.ErrCancelled }

		_, err := setupBusiness(tenant, businessOptions(""), deps)

		require.ErrorIs(t, err, portals.ErrCancelled)
		assert.Empty(t, tenant.Clients())
		assert.Empty(t, tenant.Portals())
	})

	t.Run("it asks for the test user before the portal is created", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|user1", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme-corp", "auth0|user1")
		seedAdminRole(tenant)
		tenant.AssignRole("org_1", "auth0|user1", "rol_admin")

		deps, _, prompter := interactiveDeps(tenant, answers{})

		type asked struct {
			question      string
			portalsBefore int
		}

		var questions []asked

		prompter.AskFunc = func(question, _, _ string) (string, error) {
			questions = append(questions, asked{question, len(tenant.Portals())})
			return "jane@acme.com", nil
		}

		result, err := setupBusiness(tenant, businessOptions(""), deps)

		require.NoError(t, err)
		assert.Equal(t, []asked{{"Enter the test user's email or user ID (press Enter to skip)", 0}}, questions)
		assert.Len(t, tenant.Portals(), 1)
		assert.Equal(t, "org_1", result.Organization.ID)
	})
}

func TestSetup_BusinessPortalWithoutATestUser(t *testing.T) {
	t.Parallel()

	t.Run("skipping the test user links to the organization ID that was typed", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, reporter, _ := interactiveDeps(tenant, answers{asks: []string{"", "org_42"}})

		result, err := setupBusiness(tenant, businessOptions(""), deps)

		require.NoError(t, err)
		assert.Equal(t, "https://acme.us.auth0.com/portals/acme-admin?organization=org_42", result.Portal.URL)
		assert.Nil(t, result.Organization, "no membership is checked without a user")
		assert.False(t, reporter.Contains("info", "Replace <ORGANIZATION_ID>"))
	})

	t.Run("leaving the organization empty keeps the placeholder in the link", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		deps, reporter, prompter := interactiveDeps(tenant, answers{})
		prompter.AskFunc = func(question, defaultValue, _ string) (string, error) {
			if strings.Contains(question, "test user") {
				return "", nil
			}

			return defaultValue, nil
		}

		result, err := setupBusiness(tenant, businessOptions(""), deps)

		require.NoError(t, err)
		assert.Equal(t, "https://acme.us.auth0.com/portals/acme-admin?organization=<ORGANIZATION_ID>", result.Portal.URL)
		assert.True(t, reporter.Contains("info", "Replace <ORGANIZATION_ID> in the link with the organization ID to test with."))
		assert.True(t, reporter.Contains("info", "Business portals only work in the context of an organization"))
		assert.True(t, reporter.Contains("info", "Without a test user, the link needs an organization."))
	})
}

func TestSetup_ChoosingTheTestUser(t *testing.T) {
	t.Parallel()

	t.Run("an email lookup keeps only the exact address, not a longer one that contains it", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedUser(tenant, "auth0|b", "jane@acme.com.au")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		deps, _, _ := newSetupDeps(tenant)

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
	})

	t.Run("an email that matches several users lets the person pick one", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedUser(tenant, "google-oauth2|b", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "google-oauth2|b")
		deps, _, _ := interactiveDeps(tenant, answers{chooses: []string{"google-oauth2|b"}, confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
	})

	t.Run("from a list of users with the same email, trying another looks the user up again", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedUser(tenant, "auth0|b", "jane@acme.com")
		seedUser(tenant, "auth0|other", "other@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|other")
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"auth0|other"},
			chooses:  []string{"Try another"},
			confirms: []bool{false},
		})

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
	})

	t.Run("an unknown email asks again instead of stopping", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		deps, reporter, _ := interactiveDeps(tenant, answers{asks: []string{"jane@acme.com"}, confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("nobody@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
		assert.True(t, reporter.Contains("warn", `No user matches "nobody@acme.com".`))
	})

	t.Run("a user ID that matches nobody asks again instead of stopping", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		deps, _, _ := interactiveDeps(tenant, answers{asks: []string{"auth0|a"}, confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("auth0|nobody"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
	})

	t.Run("without a terminal an email that matches several users stops with the IDs to use", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedUser(tenant, "auth0|b", "jane@acme.com")
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, `More than one user has the email "jane@acme.com"`, err.Error())
		assert.Equal(t, []string{"Rerun with the user ID instead: auth0|a, auth0|b"}, portals.FixesOf(err))
	})
}

func TestSetup_ChoosingTheOrganization(t *testing.T) {
	t.Parallel()

	t.Run("a user in several organizations can pick the one the portal is for", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		seedOrganization(tenant, "org_2", "labs", "auth0|a")
		deps, _, _ := interactiveDeps(tenant, answers{chooses: []string{"labs"}, confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_2", result.Organization.ID)
	})

	t.Run("an organization can be given by name when the user has several", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		seedOrganization(tenant, "org_2", "labs", "auth0|a")
		deps, _, _ := newSetupDeps(tenant)

		options := businessOptions("auth0|a")
		options.Organization = "labs"

		result, err := setupBusiness(tenant, options, deps)

		require.NoError(t, err)
		assert.Equal(t, "org_2", result.Organization.ID)
	})

	t.Run("without a terminal, several organizations stop with the IDs to choose from", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		seedOrganization(tenant, "org_2", "labs", "auth0|a")
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, "jane@acme.com belongs to 2 organizations", err.Error())
		assert.Equal(t, []string{"Rerun with --organization <id>: org_1, org_2"}, portals.FixesOf(err))
	})

	t.Run("without a terminal, an organization the user is not in stops with the command to add them", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")
		seedOrganization(tenant, "org_2", "labs")
		deps, _, _ := newSetupDeps(tenant)

		options := businessOptions("auth0|a")
		options.Organization = "org_2"

		_, err := setupBusiness(tenant, options, deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Contains(t, strings.Join(portals.FixesOf(err), " "), "organizations/org_2/members")
	})

	t.Run("without a terminal, a user in no organization stops with the command to add them", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|user1", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme-corp")
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Contains(t, portals.FixesOf(err)[0], `auth0 api post "organizations/<org-id>/members"`)
		assert.Empty(t, tenant.Portals(), "no portal is created before the test user is checked")
	})

	t.Run("a user in no organization can join an existing one after confirmation", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme")
		deps, _, _ := interactiveDeps(tenant, answers{chooses: []string{"acme"}, confirms: []bool{true, false}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.Equal(t, "org_1", result.Organization.ID)
		assert.Contains(t, tenant.Members("org_1"), "auth0|a", "the user was added as a member")
	})

	t.Run("declining to add a user to an organization adds nobody", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme")
		deps, _, _ := interactiveDeps(tenant, answers{chooses: []string{"acme"}, confirms: []bool{false}})

		_, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.NotContains(t, tenant.Members("org_1"), "auth0|a")
		assert.Empty(t, tenant.Portals())
	})

	t.Run("an organization with no enabled connection is refused", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|user1", "jane@acme.com")
		tenant.AddOrganization(portals.Organization{ID: "org_1", Name: "acme-corp"}, "auth0|user1")
		deps, _, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Contains(t, err.Error(), "no enabled connections")
		assert.Contains(t, portals.FixesOf(err)[0], `organizations/org_1/enabled_connections`)
	})
}

func TestSetup_CreatingAnOrganization(t *testing.T) {
	t.Parallel()

	t.Run("a user with no organization can create one, with a database connection and themselves as a member", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedDatabase(tenant)
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "acme-admins"},
			chooses:  []string{"None of these"},
			confirms: []bool{true, false},
		})

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)

		organizations := tenant.Organizations()
		require.Len(t, organizations, 1)
		assert.Equal(t, "Acme Admins", organizations[0].DisplayName)
		assert.Equal(t, organizations[0].ID, result.Organization.ID)
		assert.Contains(t, tenant.Members(organizations[0].ID), "auth0|a")

		connections, err := tenant.ListOrganizationConnections(context.Background(), organizations[0].ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"con_db"}, connections)
	})

	t.Run("declining the confirmation creates nothing", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedDatabase(tenant)
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "acme-admins"},
			chooses:  []string{"None of these"},
			confirms: []bool{false},
		})

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, "No organization was created", err.Error())
		assert.Empty(t, tenant.Organizations())
	})

	t.Run("a taken name asks for another name, with a suggestion", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		tenant.AddOrganization(portals.Organization{ID: "org_taken", Name: "acme-admins", DisplayName: "Acme"})
		seedDatabase(tenant)
		deps, reporter, prompter := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "acme-admins", "acme-admins-2"},
			chooses:  []string{"None of these"},
			confirms: []bool{true, true, false},
		})

		var suggestions []string

		scripted := prompter.AskFunc
		prompter.AskFunc = func(question, defaultValue, example string) (string, error) {
			suggestions = append(suggestions, question+"="+defaultValue)
			return scripted(question, defaultValue, example)
		}

		result, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.True(t, reporter.Contains("warn", `The organization name "acme-admins" is already used in this tenant.`))
		assert.Equal(t, []string{
			"Organization display name=",
			"Organization name=acme-admins",
			"Organization name=acme-admins-2",
		}, suggestions)

		for _, organization := range tenant.Organizations() {
			if organization.ID == result.Organization.ID {
				assert.Equal(t, "acme-admins-2", organization.Name)
				assert.Equal(t, "Acme Admins", organization.DisplayName)
			}
		}
	})

	t.Run("a name with invalid characters asks again", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedDatabase(tenant)
		deps, reporter, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "Acme Admins!", "acme-admins"},
			chooses:  []string{"None of these"},
			confirms: []bool{true, false},
		})

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		require.NoError(t, err)
		assert.Equal(t, "acme-admins", tenant.Organizations()[0].Name)
		assert.True(t, reporter.Contains("error", "Use lowercase letters, numbers, hyphens and underscores, up to 50 characters."))
	})

	t.Run("a conflict after the organization exists is not treated as a name clash", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedDatabase(tenant)
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Corp", "acme-corp"},
			chooses:  []string{"None of these"},
			confirms: []bool{true},
		})
		deps.API = conflictingConnectionTenant{Tenant: tenant}

		_, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.Error(t, err)
		assert.True(t, portals.IsStatus(err, http.StatusConflict))
		assert.Len(t, tenant.Organizations(), 1, "asking for a new name would have created a second organization")
	})

	t.Run("a tenant with no database connection stops with what to do", func(t *testing.T) {
		t.Parallel()

		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		deps, _, _ := interactiveDeps(tenant, answers{
			asks:     []string{"Acme Admins", "acme-admins"},
			chooses:  []string{"None of these"},
			confirms: []bool{true},
		})

		_, err := setupBusiness(tenant, businessOptions("jane@acme.com"), deps)

		var access *portals.BusinessAccessError
		require.ErrorAs(t, err, &access)
		assert.Equal(t, "This tenant has no database connection to sign in with", err.Error())
	})
}

// conflictingConnectionTenant is a tenant where enabling a connection on an organization conflicts.
type conflictingConnectionTenant struct {
	*portalstest.Tenant
}

func (conflictingConnectionTenant) EnableOrganizationConnection(context.Context, string, string) error {
	return &portals.APIError{Status: http.StatusConflict, Message: "The connection is already enabled."}
}

func TestSetup_ABusinessSetupReadsTheDomainBeforeItCreatesAnything(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	seedDatabase(tenant)
	seedUser(tenant, "auth0|a", "jane@acme.com")
	tenant.FailNext("GET custom-domains/default", http.StatusForbidden)

	deps, _, _ := interactiveDeps(tenant, answers{
		asks:     []string{"Acme Admins", "acme-admins"},
		chooses:  []string{"None of these"},
		confirms: []bool{true, true},
	})

	_, err := portals.Setup(context.Background(), businessOptions("jane@acme.com"), deps)

	require.Error(t, err)
	assert.True(t, portals.IsStatus(err, http.StatusForbidden), "got %v", err)

	ctx := context.Background()

	organizations, err := tenant.ListOrganizations(ctx)
	require.NoError(t, err)
	assert.Empty(t, organizations, "no organization is left behind")

	roles, err := tenant.ListRoles(ctx)
	require.NoError(t, err)
	assert.Empty(t, roles, "no role is left behind")

	servers, err := tenant.ListResourceServers(ctx)
	require.NoError(t, err)
	assert.Empty(t, servers, "nothing was created")
}

func TestSetup_TheRoleOfTheTestUser(t *testing.T) {
	t.Parallel()

	member := func() *portalstest.Tenant {
		tenant := portalstest.New()
		seedUser(tenant, "auth0|a", "jane@acme.com")
		seedOrganization(tenant, "org_1", "acme", "auth0|a")

		return tenant
	}

	t.Run("a role that grants every organization scope is accepted without an offer", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		seedAdminRole(tenant)
		tenant.AssignRole("org_1", "auth0|a", "rol_admin")
		deps, reporter, prompter := interactiveDeps(tenant, answers{})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.True(t, result.Organization.HasRole)
		assert.Empty(t, prompter.Questions())
		assert.False(t, reporter.Contains("warn", "lacks"))
	})

	t.Run("a user with no role can be given one after confirmation", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		seedAdminRole(tenant)
		deps, _, _ := interactiveDeps(tenant, answers{chooses: []string{"Organization - Admin"}, confirms: []bool{true}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.True(t, result.Organization.HasRole)

		roles, err := tenant.ListMemberRoles(context.Background(), "org_1", "auth0|a")
		require.NoError(t, err)
		assert.Equal(t, []portals.Role{{ID: "rol_admin", Name: "Organization - Admin"}}, roles)
	})

	t.Run("declining the role keeps going, and the user has no role", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		seedAdminRole(tenant)
		deps, _, _ := interactiveDeps(tenant, answers{confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.False(t, result.Organization.HasRole)

		roles, err := tenant.ListMemberRoles(context.Background(), "org_1", "auth0|a")
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("a role that lacks organization scopes is a warning that names them", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		tenant.AddRole(portals.Role{ID: "rol_reader", Name: "Organization - Reader"}, permissionsOf([]string{"read:my_org:details", "read:my_org:domains"})...)
		tenant.AssignRole("org_1", "auth0|a", "rol_reader")
		deps, reporter, _ := newSetupDeps(tenant)

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.False(t, result.Organization.HasRole)
		assert.True(t, reporter.Contains("warn", "lacks read permissions"))
		assert.True(t, reporter.Contains("warn", "read:my_org:configuration"))
		assert.True(t, reporter.Contains("warn", "lacks write permissions"))
		assert.True(t, reporter.Contains("warn", "update:my_org:details"))
		assert.True(t, reporter.Contains("warn", "delete:my_org:domains"))
	})

	t.Run("a role with read but no write scopes warns that changes cannot be saved", func(t *testing.T) {
		t.Parallel()

		tenant := member()

		var reads []string

		for _, scope := range portals.MyOrganizationScopes() {
			if strings.HasPrefix(scope, "read:") {
				reads = append(reads, scope)
			}
		}

		tenant.AddRole(portals.Role{ID: "rol_viewer", Name: "Organization - Viewer"}, permissionsOf(reads)...)
		tenant.AssignRole("org_1", "auth0|a", "rol_viewer")
		deps, reporter, _ := newSetupDeps(tenant)

		_, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.True(t, reporter.Contains("warn", "lacks write permissions"))
		assert.False(t, reporter.Contains("warn", "lacks read permissions"))
		assert.True(t, reporter.Contains("warn", "changes cannot be saved"))
	})

	t.Run("with no role that grants the missing permissions the offer is to create the portal role, and declining keeps the user without one", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		tenant.AddRole(portals.Role{ID: "rol_reader", Name: "Organization - Reader"}, permissionsOf([]string{"read:my_org:details"})...)
		deps, _, _ := interactiveDeps(tenant, answers{confirms: []bool{false}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.False(t, result.Organization.HasRole)

		for _, role := range tenant.Roles() {
			assert.NotEqual(t, "Universal Portals - Admin", role.Name)
		}
	})

	t.Run("accepting the offer creates Universal Portals - Admin with only the portal permissions and assigns it", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		deps, _, _ := interactiveDeps(tenant, answers{confirms: []bool{true}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.True(t, result.Organization.HasRole)

		var created portals.Role

		for _, role := range tenant.Roles() {
			if role.Name == "Universal Portals - Admin" {
				created = role
			}
		}

		require.NotEmpty(t, created.ID, "the role was created")

		names := make([]string, 0)
		for _, permission := range tenant.RolePermissions(created.ID) {
			names = append(names, permission.Name)
			assert.Equal(t, "https://acme.us.auth0.com/my-org/", permission.ResourceServerIdentifier)
		}

		assert.ElementsMatch(t, portals.MyOrganizationScopes(), names)

		roles, err := tenant.ListMemberRoles(context.Background(), "org_1", "auth0|a")
		require.NoError(t, err)
		assert.Equal(t, []portals.Role{created}, roles)
	})

	t.Run("the portal role is reused, and only its missing permissions are added", func(t *testing.T) {
		t.Parallel()

		tenant := member()
		tenant.AddRole(portals.Role{ID: "rol_portal", Name: "Universal Portals - Admin"}, permissionsOf([]string{"read:my_org:details"})...)
		deps, _, _ := interactiveDeps(tenant, answers{confirms: []bool{true, true}, chooses: []string{`Create "Universal Portals - Admin"`}})

		result, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

		require.NoError(t, err)
		assert.True(t, result.Organization.HasRole)
		assert.Len(t, tenant.Roles(), 1, "no second role with the same name")
		assert.Len(t, tenant.RolePermissions("rol_portal"), len(portals.MyOrganizationScopes()), "the permission it had is not added again")
	})
}

// rendezvousTenant makes the permission reads of the roles named "rol_slow_*" wait for each other: a
// read returns once another one is in flight, or after the timeout. Reads sent one after the other
// can never meet, so the tenant tells whether they went out together.
type rendezvousTenant struct {
	*portalstest.Tenant

	inFlight atomic.Int32
	met      chan struct{}
	once     sync.Once
}

func (r *rendezvousTenant) ListRolePermissions(ctx context.Context, roleID string) ([]portals.Permission, error) {
	if strings.HasPrefix(roleID, "rol_slow_") {
		if r.inFlight.Add(1) > 1 {
			r.once.Do(func() { close(r.met) })
		}

		select {
		case <-r.met:
		case <-time.After(500 * time.Millisecond):
		}

		r.inFlight.Add(-1)
	}

	return r.Tenant.ListRolePermissions(ctx, roleID)
}

// A tenant can have many roles, and looking for the ones that fix a warning reads the permissions of
// each, so the reads go out together like the other reads of setup and the audit.
func TestSetup_TheRolesThatFixAWarningAreReadTogether(t *testing.T) {
	t.Parallel()

	tenant := portalstest.New()
	seedUser(tenant, "auth0|a", "jane@acme.com")
	seedOrganization(tenant, "org_1", "acme", "auth0|a")
	tenant.AddRole(portals.Role{ID: "rol_reader", Name: "Organization - Reader"}, permissionsOf([]string{"read:my_org:details"})...)
	tenant.AssignRole("org_1", "auth0|a", "rol_reader")

	for _, id := range []string{"rol_slow_1", "rol_slow_2", "rol_slow_3"} {
		tenant.AddRole(portals.Role{ID: id, Name: "Other " + id}, permissionsOf([]string{"read:my_org:details"})...)
	}

	gated := &rendezvousTenant{Tenant: tenant, met: make(chan struct{})}
	deps, _, _ := interactiveDeps(tenant, answers{confirms: []bool{false}})
	deps.API = gated

	_, err := setupBusiness(tenant, businessOptions("auth0|a"), deps)

	require.NoError(t, err)

	select {
	case <-gated.met:
	default:
		t.Fatal("the permissions of the roles were read one after the other")
	}
}
