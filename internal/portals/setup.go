package portals

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SetupOptions are the choices that shape a portal.
type SetupOptions struct {
	Template Template
	Name     string
	Slug     string
	// UserQuery is the email or ID of the user who will test a business portal.
	UserQuery string
	// Organization is the ID or name of the organization of that user, when they have several.
	Organization string
}

// SetupDeps is what setup works with.
type SetupDeps struct {
	API API
	// Prober sends the permission checks. A dry run passes the real tenant here, because the checks
	// change nothing. It defaults to API.
	Prober   Prober
	Reporter Reporter
	Prompter Prompter
	// Interactive says whether there is somebody to ask.
	Interactive  bool
	TenantDomain string
}

func (d SetupDeps) prober() Prober {
	if d.Prober != nil {
		return d.Prober
	}

	return d.API
}

// SetupResult is what setup did. The order of the fields is the order of its JSON.
type SetupResult struct {
	Template    Template          `json:"template"`
	Portal      PortalResult      `json:"portal"`
	Application ApplicationResult `json:"application"`
	Vault       VaultResult       `json:"vault"`
	Forms       FormsResult       `json:"forms"`
	Resources   []Resource        `json:"resources"`
	// Organization is set for a business portal whose test user was chosen.
	Organization *OrganizationResult `json:"organization,omitempty"`
}

// PortalResult is the portal.
type PortalResult struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	URL    string `json:"url"`
	Status Status `json:"status"`
}

// ApplicationResult is the application the portal signs users in with.
type ApplicationResult struct {
	ClientID string `json:"clientId"`
	Status   Status `json:"status"`
}

// VaultResult is the vault connection of the forms.
type VaultResult struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
}

// FormResult is a form.
type FormResult struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
}

// FormsResult are the forms of the portal.
type FormsResult struct {
	PersonalInfo             FormResult `json:"personalInfo"`
	CommunicationPreferences FormResult `json:"communicationPreferences"`
	PrivacyConsent           FormResult `json:"privacyConsent"`
}

// Resource is a resource setup created, reused or updated.
type Resource struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Status Status `json:"status"`
}

// OrganizationResult is the organization a business portal opens in.
type OrganizationResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	HasRole bool   `json:"hasRole"`
}

// Setup creates a portal, with the resources it needs, or finishes the setup of one that a failed
// run left half done. Every resource is looked for by a stable key before it is created, and the
// portal comes last, so rerunning the same command resumes.
func Setup(ctx context.Context, options SetupOptions, deps SetupDeps) (SetupResult, error) {
	if !options.Template.IsValid() {
		return SetupResult{}, &UsageError{
			Message: fmt.Sprintf("unknown template %q", options.Template),
			Hints:   []string{"Use --template consumer or --template business."},
		}
	}

	run := &setupRun{options: options, deps: deps, tenant: deps.TenantDomain}

	return run.execute(ctx)
}

// setupRun is one run of Setup.
type setupRun struct {
	options   SetupOptions
	deps      SetupDeps
	tenant    string
	resources []Resource
}

func (r *setupRun) report() Reporter { return r.deps.Reporter }

func (r *setupRun) record(kind, id string, status Status) {
	r.resources = append(r.resources, Resource{Type: kind, ID: id, Status: status})
}

// task runs work behind a spinner and returns what it made.
func task[T any](reporter Reporter, title string, work func() (T, error)) (T, error) {
	var value T

	err := reporter.Task(title, func() error {
		var workErr error
		value, workErr = work()

		return workErr
	})

	return value, err
}

func (r *setupRun) execute(ctx context.Context) (SetupResult, error) {
	options := r.options

	if err := r.checkPermissions(ctx); err != nil {
		return SetupResult{}, err
	}

	if !IsValidSlug(options.Slug) {
		return SetupResult{}, &UsageError{
			Message: fmt.Sprintf("The slug %q must match %s", options.Slug, SlugPattern),
			Hints:   []string{"Use --slug with lowercase letters, digits and hyphens, starting with a letter."},
		}
	}

	// A run that cannot ask has to be told the user up front.
	if options.Template == TemplateBusiness && options.UserQuery == "" && !r.deps.Interactive {
		return SetupResult{}, &BusinessAccessError{
			Message: "A Business portal needs the user who will test it",
			Hints:   []string{"Rerun with --user <email or user id>."},
		}
	}

	appName := portalName(options.Name)

	slug, err := r.availableSlug(ctx, options.Slug, appName)
	if err != nil {
		return SetupResult{}, err
	}

	// The test user, or the organization of the link, is settled before anything is created.
	var link testLink

	if options.Template == TemplateBusiness {
		if link, err = r.chooseTestLink(ctx); err != nil {
			return SetupResult{}, err
		}
	}

	host, err := resolvePortalHost(ctx, r.deps.API, r.tenant)
	if err != nil {
		return SetupResult{}, err
	}

	if err := r.setUpResourceServers(ctx); err != nil {
		return SetupResult{}, err
	}

	client, err := r.setUpApplication(ctx, appName, slug, host)
	if err != nil {
		return SetupResult{}, err
	}

	vault, forms, err := r.setUpForms(ctx)
	if err != nil {
		return SetupResult{}, err
	}

	portal, err := r.setUpPortal(ctx, slug, client, forms)
	if err != nil {
		return SetupResult{}, err
	}

	if link.isPlaceholder {
		r.report().Info(fmt.Sprintf("Replace %s in the link with the organization ID to test with.", organizationPlaceholder))
	}

	return SetupResult{
		Template: options.Template,
		Portal: PortalResult{
			ID:     portal.value.ID,
			Slug:   portal.value.Slug,
			URL:    portalURL(host.domain, portal.value.Slug, link.organizationID),
			Status: portal.status,
		},
		Application: ApplicationResult{ClientID: client.value.clientID, Status: client.status},
		Vault:       VaultResult{ID: vault.value, Status: vault.status},
		Forms: FormsResult{
			PersonalInfo:             formResult(forms[FormPersonalInfo]),
			CommunicationPreferences: formResult(forms[FormCommunicationPreferences]),
			PrivacyConsent:           formResult(forms[FormPrivacyConsent]),
		},
		Resources:    r.resources,
		Organization: link.result(),
	}, nil
}

func formResult(outcome formOutcome) FormResult {
	return FormResult{ID: outcome.ref.id, Status: outcome.status}
}

// checkPermissions sends the checks before anything is created, so a missing permission is reported
// once and nothing is left half done.
func (r *setupRun) checkPermissions(ctx context.Context) error {
	r.report().Section("Permissions")

	result, err := task(r.report(), "Checking permissions", func() (PreflightResult, error) {
		return RunPreflight(ctx, r.deps.prober(), SetupProbes(r.options.Template), r.tenant)
	})
	if err != nil {
		return err
	}

	// The error lists the scopes and the fixes once, so they are not shown here as well.
	if err := result.Err(r.tenant); err != nil {
		return err
	}

	for _, issue := range result.Unexpected {
		detail := ""
		if !strings.Contains(issue.Message, fmt.Sprint(issue.Status)) {
			detail = fmt.Sprintf(" (HTTP %d)", issue.Status)
		}

		r.report().Warn(fmt.Sprintf("Could not verify %q: %s%s", issue.Probe, issue.Message, detail))
	}

	if len(result.Unexpected) > 0 {
		r.report().Result("No permission is missing, but the checks above could not be verified")
	} else {
		r.report().Result("The CLI session has every permission setup needs")
	}

	return nil
}

// availableSlug runs before anything is created: a slug used by another portal is fixed here and
// not after the application and the forms exist. A portal that already uses the application of this
// setup is a rerun, so it passes.
func (r *setupRun) availableSlug(ctx context.Context, slug, appName string) (string, error) {
	current := slug

	for {
		portal, err := findPortalBySlug(ctx, r.deps.API, current)
		if err != nil {
			return "", err
		}

		if portal == nil {
			return current, nil
		}

		app, err := findClientByName(ctx, r.deps.API, appName)
		if err != nil {
			return "", err
		}

		if app != nil && portal.Client != nil && portal.Client.ClientID == app.ID {
			return current, nil
		}

		if !r.deps.Interactive {
			return "", &SlugTakenError{Slug: current}
		}

		r.report().Warn(fmt.Sprintf("The slug %q is already taken.", current))

		current, err = r.askValidSlug(current + "-2")
		if err != nil {
			return "", err
		}
	}
}

func (r *setupRun) askValidSlug(suggestion string) (string, error) {
	for {
		answer, err := r.deps.Prompter.Ask("Slug", suggestion, "")
		if err != nil {
			return "", err
		}

		if IsValidSlug(answer) {
			return answer, nil
		}

		r.report().Error("The slug must start with a lowercase letter and use only lowercase letters, digits and hyphens.")
	}
}

func (r *setupRun) setUpResourceServers(ctx context.Context) error {
	r.report().Section("Resource servers")

	for _, server := range resourceServersFor(r.options.Template, r.tenant) {
		outcome, err := task(r.report(), "Checking "+server.Name, func() (ensured[string], error) {
			return ensureResourceServer(ctx, r.deps.API, server)
		})
		if err != nil {
			return err
		}

		r.record("resource_server", outcome.value, outcome.status)
		r.report().Item(fmt.Sprintf("%q", server.Name), outcome.status.describe())
	}

	r.report().Result("Resource servers ready")

	return nil
}

func resourceServersFor(template Template, tenantDomain string) []ResourceServerSpec {
	account := ResourceServerSpec{
		Name:       "Auth0 My Account API",
		Identifier: myAccountAudience(tenantDomain),
		SkipConsentForVerifiableFirstPartyClients: true,
		TokenDialect: "rfc9068_profile",
	}
	if template == TemplateConsumer {
		return []ResourceServerSpec{account}
	}

	organization := ResourceServerSpec{
		Name:       "Auth0 My Organization API",
		Identifier: myOrganizationAudience(tenantDomain),
		SkipConsentForVerifiableFirstPartyClients: true,
		TokenDialect: "rfc9068_profile",
	}

	return []ResourceServerSpec{organization, account}
}

func (r *setupRun) setUpApplication(ctx context.Context, appName, slug string, host portalHost) (ensured[credentials], error) {
	r.report().Section("Application")

	client, err := task(r.report(), "Creating application", func() (ensured[credentials], error) {
		return ensurePortalClient(ctx, r.deps.API, portalApplication{
			name:           appName,
			slug:           slug,
			template:       r.options.Template,
			tenantDomain:   r.tenant,
			host:           host.domain,
			isCustomDomain: host.isCustom,
		})
	})
	if err != nil {
		return ensured[credentials]{}, err
	}

	r.record("application", client.value.clientID, client.status)
	r.report().Item(fmt.Sprintf("%q", appName), client.status.describe())
	r.report().Reference("Application", client.value.clientID, ApplicationURL(r.tenant, client.value.clientID))

	for _, access := range audiencesFor(r.options.Template, r.tenant) {
		label := apiLabel(access.audience, r.tenant)

		grant, grantErr := task(r.report(), "Granting "+label, func() (ensured[string], error) {
			return ensureClientGrant(ctx, r.deps.API, ClientGrantSpec{
				ClientID:    client.value.clientID,
				Audience:    access.audience,
				Scope:       access.scope,
				SubjectType: "user",
			})
		})
		if grantErr != nil {
			return ensured[credentials]{}, grantErr
		}

		r.record("client_grant", grant.value, grant.status)
		r.report().Item(fmt.Sprintf("%q access", label), grant.status.describe())
	}

	r.report().Result("Application ready")

	return client, nil
}

// setUpForms prepares the forms vault, then creates the forms that write through it.
func (r *setupRun) setUpForms(ctx context.Context) (ensured[string], map[FormKey]formOutcome, error) {
	r.report().Section("Forms")

	vault, err := r.setUpFormsVault(ctx)
	if err != nil {
		return ensured[string]{}, nil, err
	}

	definitions, err := buildForms(vault.value)
	if err != nil {
		return ensured[string]{}, nil, err
	}

	outcomes, err := task(r.report(), "Creating forms", func() ([]formOutcome, error) {
		return ensureForms(ctx, r.deps.API, definitions)
	})
	if err != nil {
		return ensured[string]{}, nil, err
	}

	forms := make(map[FormKey]formOutcome, len(outcomes))

	for _, outcome := range outcomes {
		forms[outcome.key] = outcome

		r.record("form", outcome.ref.id, outcome.status)
		r.report().Item(fmt.Sprintf("%q", FormName(outcome.key)), outcome.status.describe())
		r.report().Reference("Form", outcome.ref.id, FormEditURL(r.tenant, outcome.ref.id))

		if outcome.ref.flowID != "" {
			r.report().Reference("Flow", outcome.ref.flowID, FlowEditURL(r.tenant, outcome.ref.flowID))
		}
	}

	r.report().Result("Forms ready")

	return vault, forms, nil
}

// setUpFormsVault prepares the vault shared by the forms: one machine-to-machine application and
// one vault connection for the tenant. A change of its scopes needs a refresh, which sends the setup
// again so the vault signs in with the new grants.
func (r *setupRun) setUpFormsVault(ctx context.Context) (ensured[string], error) {
	app, err := task(r.report(), "Preparing the forms vault", func() (ensured[credentials], error) {
		return ensureFormsVaultClient(ctx, r.deps.API, formsVaultName)
	})
	if err != nil {
		return ensured[string]{}, err
	}

	grant, err := ensureClientGrant(ctx, r.deps.API, ClientGrantSpec{
		ClientID:    app.value.clientID,
		Audience:    managementAudience(r.tenant),
		Scope:       formsVaultScopes,
		SubjectType: "client",
	})
	if err != nil {
		return ensured[string]{}, err
	}

	setup := VaultSetup{Domain: r.tenant, ClientID: app.value.clientID, ClientSecret: app.value.clientSecret}

	vault, err := ensureVaultConnection(ctx, r.deps.API, formsVaultName, setup)
	if err != nil {
		return ensured[string]{}, err
	}

	if grant.status == StatusUpdated && vault.status == StatusReused {
		if err := refreshVaultConnection(ctx, r.deps.API, vault.value, setup); err != nil {
			return ensured[string]{}, err
		}

		r.report().Item("Forms vault refreshed with the updated permissions", "")
	}

	r.record("forms_vault_client", app.value.clientID, app.status)
	r.record("forms_vault_grant", grant.value, grant.status)
	r.record("forms_vault", vault.value, vault.status)
	r.report().Item(fmt.Sprintf("%q", formsVaultName), vault.status.describe())
	// Vault connections have no page of their own, so the ID links to the AUTH0 app type.
	r.report().Reference("Vault connection", vault.value, VaultAppsURL(r.tenant))

	return vault, nil
}

func (r *setupRun) setUpPortal(
	ctx context.Context,
	slug string,
	client ensured[credentials],
	forms map[FormKey]formOutcome,
) (ensured[Portal], error) {
	template, err := templateFor(r.options.Template, formIDs{
		FormPersonalInfo:             forms[FormPersonalInfo].ref.id,
		FormPrivacyConsent:           forms[FormPrivacyConsent].ref.id,
		FormCommunicationPreferences: forms[FormCommunicationPreferences].ref.id,
	})
	if err != nil {
		return ensured[Portal]{}, err
	}

	r.report().Section("Portal")

	portal, err := task(r.report(), "Creating portal", func() (ensured[Portal], error) {
		return ensurePortal(ctx, r.deps.API, portalRequest(template, slug, r.options.Name, client.value))
	})
	if _, taken := asSlugTaken(err); taken {
		// No prompt can run inside the task, so the run stops. The resources created so far stay, and
		// a new name gives a new application, so the rerun needs both a new name and a new slug.
		return ensured[Portal]{}, &SlugRaceError{Slug: slug}
	}

	if err != nil {
		return ensured[Portal]{}, err
	}

	r.record("portal", portal.value.ID, portal.status)
	r.report().Item(fmt.Sprintf("%q", r.options.Name), portal.status.describe())
	r.report().Result("Portal ready")
	r.report().Reference("Portal", portal.value.ID, PortalEditURL(r.tenant, portal.value.ID))

	return portal, nil
}

// portalHost is where a portal is served from.
type portalHost struct {
	domain   string
	isCustom bool
}

// resolvePortalHost returns the custom domain of the tenant when it is ready, and the tenant domain
// otherwise.
func resolvePortalHost(ctx context.Context, store CustomDomainStore, tenantDomain string) (portalHost, error) {
	custom, err := store.ReadDefaultCustomDomain(ctx)
	if IsStatus(err, http.StatusNotFound) {
		return portalHost{domain: tenantDomain}, nil
	}

	if err != nil {
		return portalHost{}, fmt.Errorf("read the custom domain: %w", err)
	}

	if custom.Status == "ready" && custom.Domain != "" {
		return portalHost{domain: custom.Domain, isCustom: true}, nil
	}

	return portalHost{domain: tenantDomain}, nil
}

// organizationPlaceholder stays in the link of a business portal until the person replaces it with
// an organization ID.
const organizationPlaceholder = "<ORGANIZATION_ID>"

// portalURL is the address of a portal. A business portal opens in an organization.
func portalURL(host, slug, organizationID string) string {
	address := "https://" + host + "/portals/" + slug
	if organizationID == "" {
		return address
	}

	// The placeholder stays readable, so it is easy to find and replace.
	value := organizationID
	if organizationID != organizationPlaceholder {
		value = strings.ReplaceAll(url.QueryEscape(organizationID), "+", "%20")
	}

	return address + "?organization=" + value
}

// AskForMissing asks for the choices that were not given, before anything is checked or created:
// the kind of portal, its name, and its slug. The slug defaults to the one made from the name.
func AskForMissing(options SetupOptions, prompter Prompter) (SetupOptions, error) {
	if options.Template == "" {
		choices := []Choice{
			{Label: "Consumer portal", Hint: "for end-users to manage their account in a self-service way"},
			{Label: "Business portal", Hint: "for organization administrators to manage their organization in a self-service way"},
		}

		picked, err := prompter.Choose("Which kind of portal do you want to set up?", choices)
		if err != nil {
			return SetupOptions{}, err
		}

		options.Template = []Template{TemplateConsumer, TemplateBusiness}[picked]
	}

	if options.Name == "" {
		name, err := prompter.Ask("Portal name", "My Portal", "")
		if err != nil {
			return SetupOptions{}, err
		}

		options.Name = name
	}

	if options.Slug == "" {
		slug, err := prompter.Ask("URL slug", SlugFromName(options.Name), "")
		if err != nil {
			return SetupOptions{}, err
		}

		options.Slug = slug
	}

	return options, nil
}

// result is the organization of the result of setup, set when a test user was chosen.
func (l testLink) result() *OrganizationResult {
	if l.access == nil {
		return nil
	}

	return &OrganizationResult{ID: l.access.organizationID, Name: l.access.organizationName, HasRole: l.access.hasOrganizationRole}
}
