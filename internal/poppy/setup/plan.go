package setup

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// Kind is the kind of resource a change is made to.
type Kind string

// The kinds of change in a plan.
const (
	KindTenant Kind = "tenant"
	KindAPI    Kind = "api"
	KindApp    Kind = "app"
	KindFile   Kind = "file"
)

// Impact says how far a change reaches.
type Impact string

// The impacts a change can have.
const (
	ImpactTenantWide Impact = "tenant-wide"
	ImpactAPIWide    Impact = "api-wide"
	ImpactApp        Impact = "app"
	ImpactFile       Impact = "file"
)

// The names of the fields a change sets. The tenant fields are Management API
// setting names, and apply reads them back to build its request.
const (
	fieldCIMD            = "client_id_metadata_document_supported"
	fieldIss             = "authorization_response_iss_parameter_supported"
	fieldResourceProfile = "resource_parameter_profile"
	fieldDefaultAudience = "default_audience"
	// FieldCompanyDomains is exported because the text of a plan shows only the
	// company domain that is added.
	FieldCompanyDomains = "personal_agent_protocol_company_domains"
	fieldScopes         = "scopes"
	fieldOfflineAccess  = "allow_offline_access"
	fieldAgentClaims    = "agent_subject_claims"
	fieldPoppy          = "use_for_personal_agent_protocol"
	fieldGuestPolicy    = "subject_type_authorization.anonymous_user.policy"
	fieldLoginURI       = "initiate_login_uri"
	fieldBrowserJoin    = "allow_personal_agent_browser_join"
)

// ValueNone is the Before value of a setting that does not exist.
const ValueNone = "none"

// Field is one value a change sets.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Before is the value the tenant has now, ValueNone when the setting does not
	// exist. It is empty when it could not be read.
	Before string `json:"before,omitempty"`
	// Effect says what else the value changes, for a tenant-wide setting.
	Effect string `json:"effect,omitempty"`
}

// Label is the name of the setting in the text of a plan. It is false for a
// field that has none.
func (f Field) Label() (string, bool) {
	label, ok := settingLabels[f.Name]

	return label, ok
}

// IsToggle tells whether the setting is on or off.
func (f Field) IsToggle() bool { return toggleFields[f.Name] }

// toggleFields are the settings that are on or off.
var toggleFields = map[string]bool{
	fieldCIMD: true, fieldIss: true, fieldOfflineAccess: true, fieldPoppy: true, fieldBrowserJoin: true,
}

// ScopeChange is a scope of an API that the plan adds or describes.
type ScopeChange struct {
	Name        string
	Description string
	// New tells whether the API lacks the scope. Otherwise the scope exists
	// without a description, and setup gives it one.
	New bool
}

// Change is one step of the plan. It holds only differences from the current
// state.
type Change struct {
	Kind        Kind    `json:"kind"`
	Description string  `json:"description"`
	Impact      Impact  `json:"impact"`
	Fields      []Field `json:"fields"`
	// ScopeChanges lists the scopes behind the scopes field of an API change, with
	// their descriptions, for the text. The JSON keeps the scopes field only.
	ScopeChanges []ScopeChange `json:"-"`

	// The identifier of the API the change is made to, for the changes of an API.
	target string
}

// Target is the identifier of the API a change is made to, for the changes of an
// API.
func (c Change) Target() string { return c.target }

// Settings returns the fields that count as settings of the change. The scopes
// field is left out when ScopeChanges list the scopes one by one.
func (c Change) Settings() []Field {
	if len(c.ScopeChanges) == 0 {
		return c.Fields
	}

	var fields []Field
	for _, field := range c.Fields {
		if field.Name != fieldScopes {
			fields = append(fields, field)
		}
	}

	return fields
}

// Count is the number of settings the change sets, which is what the text counts
// as changes: a scope, a field or the config file each count one.
func (c Change) Count() int {
	if c.Kind == KindFile {
		return 1
	}

	return len(c.ScopeChanges) + len(c.Settings())
}

// Plan is the list of changes setup makes, in the order it applies them.
type Plan struct {
	// Tenant is the canonical domain of the tenant.
	Tenant   string   `json:"tenant"`
	Issuer   string   `json:"issuer"`
	Warnings []string `json:"warnings"`
	Changes  []Change `json:"changes"`

	// Unchanged names the Poppy settings that already have the wanted value.
	Unchanged []string `json:"unchanged,omitempty"`
	// CompanyDomain is the website host the plan is for.
	CompanyDomain string `json:"company_domain,omitempty"`
	// APIs are the identifiers of the APIs, and App is the name of the web app.
	APIs          []string `json:"apis,omitempty"`
	App           string   `json:"app,omitempty"`
	AppFirstParty bool     `json:"app_first_party,omitempty"`
	// NewFile tells that the config file does not exist yet.
	NewFile bool `json:"new_file,omitempty"`

	// The kind of each warning, one for each entry of Warnings: WarningIssuer,
	// WarningAPI or empty.
	warningKinds []string
}

// The kinds of warning, which tell the text how to label it.
const (
	WarningIssuer = "issuer"
	WarningAPI    = "api"
)

// Count is the number of changes of the plan, counting each setting.
func (p Plan) Count() int {
	n := 0
	for _, change := range p.Changes {
		n += change.Count()
	}

	return n
}

// WarningKind returns the kind of the warning at index i: WarningIssuer,
// WarningAPI or empty.
func (p Plan) WarningKind(i int) string {
	if i < len(p.warningKinds) {
		return p.warningKinds[i]
	}

	return ""
}

// Empty tells whether there is nothing to change.
func (p Plan) Empty() bool {
	return len(p.Changes) == 0
}

// Scopes returns the Management API scopes needed to apply the plan: the read
// scopes, plus a write scope for each kind of change in the plan. The list is
// sorted.
func (p Plan) Scopes() []string {
	scopes := slices.Clone(ReadScopes)

	for _, change := range p.Changes {
		switch change.Kind {
		case KindTenant:
			scopes = append(scopes, "update:tenant_settings")
		case KindAPI:
			scopes = append(scopes, "update:resource_servers")
		case KindApp:
			scopes = append(scopes, "update:clients")
		}
	}

	slices.Sort(scopes)

	return slices.Compact(scopes)
}

// settingLabels are the names of the settings in the text of a plan.
var settingLabels = map[string]string{
	fieldCIMD:            "CIMD support",
	fieldIss:             "iss in sign-in redirects",
	fieldResourceProfile: "resource parameter profile",
	fieldDefaultAudience: "default audience",
	FieldCompanyDomains:  "company domain",
	fieldScopes:          "scopes",
	fieldOfflineAccess:   "offline access",
	fieldAgentClaims:     "agent subject claims",
	fieldPoppy:           "Poppy API",
	fieldGuestPolicy:     "guest policy",
	fieldLoginURI:        "Application Login URI",
	fieldBrowserJoin:     "browser join",
}

// settings collects the fields a change sets, and the names of the settings that
// already have the wanted value.
type settings struct {
	fields    []Field
	unchanged []string
	// Suffix is added to the name of a correct setting, to tell the APIs apart.
	suffix string
}

// check adds the field when the setting is not right yet, else records that it
// is correct.
func (s *settings) check(right bool, field Field) {
	if right {
		s.unchanged = append(s.unchanged, settingLabels[field.Name]+s.suffix)

		return
	}

	s.fields = append(s.fields, field)
}

// Build works out the changes needed to bring the tenant to the state setup
// wants, and nothing else. A part that is off (no API, no web app) has no change,
// and the tenant settings do not include the default audience without an API. It
// stops when the config file cannot be derived, or when the derived file is not
// valid: the file is written last, so the plan must not start changes that
// could never be finished. An invalid file is a *config.ValidationError.
func Build(state *State, in Inputs) (Plan, error) {
	scopes, err := resolveScopes(state, in)
	if err != nil {
		return Plan{}, err
	}

	cfg, err := derive(state, in, scopes)
	if err != nil {
		return Plan{}, err
	}

	if violations := cfg.Validate(); len(violations) > 0 {
		return Plan{}, &config.ValidationError{File: in.Out, Violations: violations}
	}

	plan := Plan{
		Tenant:        state.Domain,
		Issuer:        state.Issuer(),
		Changes:       []Change{},
		CompanyDomain: in.CompanyDomain,
		App:           state.App.Name,
		AppFirstParty: state.App.FirstParty,
		NewFile:       state.File == nil,
	}
	plan.Warnings, plan.warningKinds = warnings(state, in)

	for _, api := range state.APIs {
		plan.APIs = append(plan.APIs, api.Identifier)
	}

	change, unchanged, ok := tenantChange(state, in)
	plan.Unchanged = append(plan.Unchanged, unchanged...)
	if ok {
		plan.Changes = append(plan.Changes, change)
	}

	for _, api := range state.APIs {
		change, unchanged, ok := apiChange(api, scopes[api.Identifier], len(state.APIs) > 1)
		plan.Unchanged = append(plan.Unchanged, unchanged...)
		if ok {
			plan.Changes = append(plan.Changes, change)
		}
	}

	if !in.NoWeb {
		change, unchanged, ok = appChange(state.App, in)
		plan.Unchanged = append(plan.Unchanged, unchanged...)
		if ok {
			plan.Changes = append(plan.Changes, change)
		}
	}

	if len(config.Diff(state.File, cfg)) > 0 {
		plan.Changes = append(plan.Changes, Change{
			Kind:        KindFile,
			Description: "write " + in.Out,
			Impact:      ImpactFile,
			Fields:      []Field{},
		})
	}

	return plan, nil
}

// warnings returns the warnings of the plan and the kind of each.
func warnings(state *State, in Inputs) (texts, kinds []string) {
	for _, api := range state.APIs {
		if config.CheckHTTPS(api.Identifier) != nil {
			texts = append(texts, fmt.Sprintf("%s is not an https URL, and Poppy APIs normally need one.", api.Identifier))
			kinds = append(kinds, WarningAPI)
		}

		// The config file has no identifier per entry: config finds the entry
		// of an API by the host of its URL.
		if origin, url := config.Origin(api.Identifier), in.APIURLs[api.Identifier]; origin != "" && url != "" && config.Origin(url) != origin {
			texts = append(texts, fmt.Sprintf(
				"The OpenAPI URL %s is not on the host of %s. auth0 poppy config finds the entry of an API in poppy.json by that host, so it will not find this one. Serve the document on the host of the API.",
				url, api.Identifier,
			))
			kinds = append(kinds, WarningAPI)
		}
	}

	if state.usesCanonicalDomain() {
		texts = append(texts, fmt.Sprintf(
			"The tenant has no verified custom domain, so agents will see the Auth0 domain %s as the issuer. Changing the issuer later signs every user out.",
			state.Domain,
		))
		kinds = append(kinds, WarningIssuer)
	}

	return texts, kinds
}

func tenantChange(state *State, in Inputs) (Change, []string, bool) {
	tenant := state.Tenant
	var s settings

	s.check(tenant.CIMD, Field{Name: fieldCIMD, Value: "true", Before: "false"})
	s.check(tenant.IssParameter, Field{Name: fieldIss, Value: "true", Before: "false", Effect: "every app's redirects"})
	s.check(tenant.ResourceParameterProfile == "compatibility",
		Field{Name: fieldResourceProfile, Value: "compatibility", Before: cmp.Or(tenant.ResourceParameterProfile, ValueNone)})

	if len(state.APIs) == 1 {
		right := tenant.DefaultAudience == state.APIs[0].Identifier
		if tenant.DefaultAudience == "" || right {
			s.check(right, Field{Name: fieldDefaultAudience, Value: state.APIs[0].Identifier, Before: ValueNone, Effect: "apps that send no audience"})
		}
	}

	domains := companyDomainsWith(tenant.CompanyDomains, in.CompanyDomain)
	s.check(HasCompanyDomain(tenant.CompanyDomains, in.CompanyDomain),
		Field{Name: FieldCompanyDomains, Value: strings.Join(domains, ", "), Before: cmp.Or(strings.Join(tenant.CompanyDomains, ", "), ValueNone), Effect: rewrittenDomains(tenant.CompanyDomains)})

	return Change{
		Kind:        KindTenant,
		Description: "tenant settings of " + state.Domain,
		Impact:      ImpactTenantWide,
		Fields:      emptyIfNil(s.fields),
	}, s.unchanged, len(s.fields) > 0
}

// rewrittenDomains names the existing company domains that are not bare hosts,
// which the update writes as bare hosts. It is empty when there are none.
func rewrittenDomains(entries []string) string {
	var changes []string
	for _, entry := range entries {
		if bare := NormalizeCompanyDomain(entry); bare != entry {
			changes = append(changes, fmt.Sprintf("%s becomes %s", entry, bare))
		}
	}

	return strings.Join(changes, ", ")
}

func apiChange(api API, scopes apiScopes, several bool) (Change, []string, bool) {
	s := settings{}
	if several {
		s.suffix = " on " + api.Identifier
	}

	var fields []Field
	if len(scopes.Changed) > 0 {
		fields = append(fields, Field{Name: fieldScopes, Value: strings.Join(scopes.Changed, ", ")})
	}
	s.fields = fields

	s.check(api.AllowOfflineAccess, Field{Name: fieldOfflineAccess, Value: "true", Before: "false"})
	s.check(api.AgentSubjectClaims == "auth0-v1", Field{Name: fieldAgentClaims, Value: "auth0-v1", Before: cmp.Or(api.AgentSubjectClaims, ValueNone)})
	s.check(api.Poppy, Field{Name: fieldPoppy, Value: "true", Before: "false"})
	s.check(api.GuestPolicy == "require_client_grant",
		Field{Name: fieldGuestPolicy, Value: "require_client_grant", Before: cmp.Or(api.GuestPolicy, ValueNone)})

	return Change{
		Kind:         KindAPI,
		Description:  describe(api.Identifier, s.fields),
		Impact:       ImpactAPIWide,
		Fields:       emptyIfNil(s.fields),
		ScopeChanges: scopes.Details,
		target:       api.Identifier,
	}, s.unchanged, len(s.fields) > 0
}

func appChange(app App, in Inputs) (Change, []string, bool) {
	var s settings

	if in.LoginURI != "" {
		s.check(in.LoginURI == app.LoginURI, Field{Name: fieldLoginURI, Value: in.LoginURI, Before: cmp.Or(app.LoginURI, ValueNone)})
	}

	s.check(app.BrowserJoin, Field{Name: fieldBrowserJoin, Value: "true", Before: "false"})

	return Change{
		Kind:        KindApp,
		Description: describe(app.Name, s.fields),
		Impact:      ImpactApp,
		Fields:      emptyIfNil(s.fields),
	}, s.unchanged, len(s.fields) > 0
}

func emptyIfNil(fields []Field) []Field {
	if fields == nil {
		return []Field{}
	}

	return fields
}

// describe summarizes the fields a change sets under the name of its target. The
// scopes field also names the scopes, so the plan text shows which are added.
func describe(target string, fields []Field) string {
	names := make([]string, len(fields))
	for i, field := range fields {
		names[i] = field.Name
		if field.Name == fieldScopes {
			names[i] += " [" + field.Value + "]"
		}
	}

	return target + ": " + strings.Join(names, ", ")
}
