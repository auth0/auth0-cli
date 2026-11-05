package setup

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/discovery"
	"github.com/auth0/auth0-cli/internal/poppy/text"
)

// maxValueRunes is the most characters of a value from the tenant that a question
// prints.
const maxValueRunes = 300

// clean makes a value from the tenant or the user safe to print on one line.
func clean(s string) string {
	return text.Cap(text.Line(s), maxValueRunes)
}

// maxCompanyDomains is the number of company domains a tenant can list.
const maxCompanyDomains = 5

// BrowserSignInScope is the reserved scope the signed-in browser join needs.
// Setup adds it to every API that lacks it.
const BrowserSignInScope = "poppy:browser-sign-in"

// BrowserSignInDescription is the description setup gives BrowserSignInScope when
// the scope is missing or has none, unless --scope gives another.
const BrowserSignInDescription = "Use the website signed in, with the other access you approve"

// ReasonScopeDescriptionRequired is the Reason of the InputError for scopes that
// have no description.
const ReasonScopeDescriptionRequired = "scope_description_required"

// maxListedScopeFixes is the most --scope examples the FIX line of a missing
// scope description spells out.
const maxListedScopeFixes = 5

// ErrGuestScopesRequired is returned when the guest grant is asked for and its
// scopes are not chosen.
var ErrGuestScopesRequired = errors.New("guest scopes are required")

// InputError is a problem with the inputs or with what the inputs point to. It
// stops the command before anything is changed.
type InputError struct {
	// Reason is a stable, machine-readable identifier.
	Reason  string
	Message string
	// Fix says what to do about it. It can be empty.
	Fix string
}

// Error returns the message followed by the FIX line.
func (e *InputError) Error() string {
	if e.Fix == "" {
		return e.Message
	}

	return e.Message + "\n  FIX  " + e.Fix
}

func invalidFlag(flag, format string, args ...any) *InputError {
	return &InputError{Reason: "invalid_flag", Message: flag + ": " + fmt.Sprintf(format, args...)}
}

// Flags holds the flag values of the setup and config commands, as given.
type Flags struct {
	CompanyDomain  string
	APIs           []string
	APIURLs        []string
	App            string
	Scopes         []string
	Hints          []string
	Name           string
	LoginURI       string
	Out            string
	GuestAPIAccess bool
	GuestScopes    []string
	GuestAllScopes bool
	ForceConfig    bool
}

// Inputs holds the validated values of the flags and of the answers to the
// questions.
type Inputs struct {
	// CompanyDomain is a bare lower-case host.
	CompanyDomain string
	// APIs are the requested APIs, by ID or identifier.
	APIs []string
	// APIURLs holds the OpenAPI document URLs by API identifier.
	APIURLs map[string]string
	// App is the client ID or name of the web app.
	App string
	// Scopes holds the scope descriptions given by name.
	Scopes         map[string]string
	Hints          map[string]config.Hints
	Name           string
	LoginURI       string
	Out            string
	GuestAPIAccess bool
	// GuestScopeNames are the scopes named for the guest grant, on every API.
	GuestScopeNames []string
	// GuestAllScopes makes the guest grant carry every scope of the API.
	GuestAllScopes bool
	// GuestScopes holds the scopes of the guest grant by API identifier, once
	// they are chosen. An API with no entry gets every scope when GuestAllScopes
	// is set, else no guest grant.
	GuestScopes map[string][]string
	ForceConfig bool
}

// Parse validates the flag values. Missing required values are not an error
// here: see RequiredQuestions.
func (f Flags) Parse() (Inputs, error) {
	in := Inputs{
		APIs:           slices.Compact(slices.DeleteFunc(slices.Clone(f.APIs), func(s string) bool { return strings.TrimSpace(s) == "" })),
		App:            strings.TrimSpace(f.App),
		Name:           f.Name,
		Out:            f.Out,
		GuestAPIAccess: f.GuestAPIAccess,
		GuestAllScopes: f.GuestAllScopes,
		ForceConfig:    f.ForceConfig,
	}

	for _, name := range f.GuestScopes {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(in.GuestScopeNames, name) {
			in.GuestScopeNames = append(in.GuestScopeNames, name)
		}
	}

	if !f.GuestAPIAccess {
		for _, given := range []struct {
			flag string
			set  bool
		}{{"--guest-scopes", len(in.GuestScopeNames) > 0}, {"--guest-all-scopes", f.GuestAllScopes}} {
			if given.set {
				err := invalidFlag(given.flag, "it only affects the guest grant, which --guest-api-access creates")
				err.Fix = "Add --guest-api-access."

				return Inputs{}, err
			}
		}
	}

	var err error

	if f.CompanyDomain != "" {
		if in.CompanyDomain, err = parseDomain(f.CompanyDomain); err != nil {
			return Inputs{}, err
		}
	}

	if f.LoginURI != "" {
		if err = checkLoginURI(f.LoginURI); err != nil {
			return Inputs{}, err
		}
		in.LoginURI = f.LoginURI
	}

	if in.APIURLs, err = parsePairs("--api-url", f.APIURLs, checkAPIURL); err != nil {
		return Inputs{}, err
	}

	if in.Scopes, err = parsePairs("--scope", f.Scopes, checkDescription); err != nil {
		return Inputs{}, err
	}

	if in.Hints, err = parseHints(f.Hints); err != nil {
		return Inputs{}, err
	}

	return in, nil
}

// parseDomain checks a company domain and lower-cases it. Only a bare host is
// accepted.
func parseDomain(s string) (string, error) {
	if strings.Contains(s, "://") {
		return "", invalidFlag("--company-domain", "%q must be a host name such as shop.example, without a scheme", s)
	}

	domain, err := discovery.NormalizeDomain(s)
	if err != nil {
		return "", invalidFlag("--company-domain", "%s", err)
	}

	return domain, nil
}

// parsePairs reads values of the form key=value into a map, checking each value.
func parsePairs(flag string, values []string, check func(flag, value string) (string, error)) (map[string]string, error) {
	pairs := make(map[string]string, len(values))

	for _, value := range values {
		key, v, found := strings.Cut(value, "=")
		key = strings.TrimSpace(key)

		if !found || key == "" {
			return nil, invalidFlag(flag, "%q must be written <name>=<value>, where the first = separates the name from the value", value)
		}

		v, err := check(flag, v)
		if err != nil {
			return nil, err
		}

		pairs[key] = v
	}

	return pairs, nil
}

func checkDescription(flag, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", invalidFlag(flag, "the description must not be empty")
	}

	return value, nil
}

func checkAPIURL(flag, value string) (string, error) {
	if !isHTTPS(value) {
		return "", invalidFlag(flag, "%q must be an https URL", value)
	}

	return value, nil
}

func checkLoginURI(value string) error {
	if !isHTTPS(value) {
		return invalidFlag("--login-uri", "%q must be an https URL", value)
	}

	return nil
}

// parseHints reads hints of the form name=changes:irreversible,personal_data:true.
func parseHints(values []string) (map[string]config.Hints, error) {
	hints := make(map[string]config.Hints, len(values))

	for _, value := range values {
		name, rest, found := strings.Cut(value, "=")
		name = strings.TrimSpace(name)

		if !found || name == "" {
			return nil, invalidFlag("--hint", "%q must be written <scope>=changes:<value>,personal_data:<true|false>", value)
		}

		var hint config.Hints
		var hasChanges, hasPersonalData bool

		for _, part := range strings.Split(rest, ",") {
			key, v, _ := strings.Cut(strings.TrimSpace(part), ":")

			switch key {
			case "changes":
				if v != "" && !slices.Contains(config.ValidChanges, v) {
					err := invalidFlag("--hint", "changes of %s must be one of %s, got %q", name, strings.Join(config.ValidChanges, ", "), v)
					err.Fix = fmt.Sprintf("Use --hint %s=changes:<%s>,personal_data:<true|false>", name, strings.Join(config.ValidChanges, "|"))

					return nil, err
				}
				hint.Changes, hasChanges = v, v != ""
			case "personal_data":
				b, err := strconv.ParseBool(v)
				if err != nil {
					return nil, invalidFlag("--hint", "personal_data of %s must be true or false, got %q", name, v)
				}
				hint.PersonalData, hasPersonalData = b, true
			default:
				return nil, invalidFlag("--hint", "unknown hint %q for %s: use changes and personal_data", key, name)
			}
		}

		if !hasChanges || !hasPersonalData {
			return nil, invalidFlag("--hint", "%s needs both changes and personal_data", name)
		}

		hints[name] = hint
	}

	return hints, nil
}

// isHTTPS tells whether s is an https URL with a host.
func isHTTPS(s string) bool {
	u, err := url.Parse(s)

	return err == nil && u.Scheme == "https" && u.Host != ""
}

// originOf returns the origin of an https URL, or "" when s is not one.
func originOf(s string) string {
	if !isHTTPS(s) {
		return ""
	}

	u, _ := url.Parse(s)

	return "https://" + u.Host
}

// Question asks for a value that is required but was not given.
type Question struct {
	// Flag names the flag that gives the value, for callers that cannot ask.
	Flag string
	// Title names what is asked, for a caller that shows it above the question.
	Title string
	// Reason is one sentence that says why the question is asked, shown under the
	// title.
	Reason string
	// Prompt labels the place where the answer is typed.
	Prompt  string
	Default string
	apply   func(*Inputs, string) error
}

// Answer validates the answer and stores it in the inputs.
func (q Question) Answer(in *Inputs, value string) error {
	return q.apply(in, strings.TrimSpace(value))
}

// RequiredQuestions returns a question for each of the company domain, an API and
// the web app that has not been given.
func RequiredQuestions(in Inputs) []Question {
	var questions []Question

	if in.CompanyDomain == "" {
		questions = append(questions, Question{
			Flag:   "--company-domain",
			Title:  "Company domain",
			Reason: "The host of your website, such as shop.example.",
			Prompt: "Domain",
			apply: func(in *Inputs, value string) (err error) {
				in.CompanyDomain, err = parseDomain(value)

				return err
			},
		})
	}

	if len(in.APIs) == 0 {
		questions = append(questions, Question{
			Flag:   "--api",
			Title:  "API",
			Reason: "The identifier or ID of the API that agents will use.",
			Prompt: "API",
			apply: func(in *Inputs, value string) error {
				in.APIs = append(in.APIs, value)

				return nil
			},
		})
	}

	if in.App == "" {
		questions = append(questions, Question{
			Flag:   "--app",
			Title:  "Web app",
			Reason: "The first-party web app that does the browser join, by client ID or name.",
			Prompt: "App",
			apply: func(in *Inputs, value string) error {
				in.App = value

				return nil
			},
		})
	}

	return questions
}

// openAPIURLGuess returns the usual place of the OpenAPI document of an API: at
// its origin. It is empty when the identifier is not an https URL.
func openAPIURLGuess(identifier string) string {
	if origin := originOf(identifier); origin != "" {
		return origin + "/openapi.json"
	}

	return ""
}

// Questions returns the next batch of questions about what the tenant state made
// necessary: the OpenAPI URL of an API that has none and the login route of the
// web app. It returns nothing when all are answered. Answer the batch, then call
// it again.
func Questions(state *State, in Inputs) []Question {
	var urls []Question

	for _, api := range state.APIs {
		identifier := api.Identifier

		if in.APIURLs[identifier] != "" || len(state.fileEntries(identifier)) > 0 {
			continue
		}

		urls = append(urls, Question{
			Flag:    "--api-url " + identifier + "=<openapi url>",
			Title:   "OpenAPI document of " + clean(identifier),
			Reason:  "Agents read it to learn the API.",
			Prompt:  "URL",
			Default: openAPIURLGuess(identifier),
			apply: func(in *Inputs, value string) error {
				value, err := checkAPIURL("--api-url", value)
				if err != nil {
					return err
				}

				if in.APIURLs == nil {
					in.APIURLs = map[string]string{}
				}
				in.APIURLs[identifier] = value

				return nil
			},
		})
	}

	if len(urls) > 0 {
		return urls
	}

	if in.LoginURI == "" && !isHTTPS(state.App.LoginURI) {
		return []Question{{
			Flag:   "--login-uri",
			Title:  "Login route of " + clean(state.App.Name),
			Reason: "The browser join sends people there to sign in. It must be an https URL.",
			Prompt: "URL",
			apply: func(in *Inputs, value string) error {
				if err := checkLoginURI(value); err != nil {
					return err
				}
				in.LoginURI = value

				return nil
			},
		}}
	}

	return nil
}

// Check validates the inputs against the state. It stops with an *InputError and
// a FIX line when the browser join cannot work or the scopes are not ready to be
// offered to agents.
func Check(state *State, in Inputs) error {
	if !state.App.FirstParty {
		return &InputError{
			Reason:  "app_not_first_party",
			Message: fmt.Sprintf("The browser join needs a first-party web app. %s is not first-party.", state.App.Name),
			Fix:     "Pass a first-party web app with --app.",
		}
	}

	identifiers := make([]string, len(state.APIs))
	for i, api := range state.APIs {
		identifiers[i] = api.Identifier
	}

	for _, identifier := range slices.Sorted(maps.Keys(in.APIURLs)) {
		if !slices.Contains(identifiers, identifier) {
			return invalidFlag("--api-url", "%s is not one of the APIs. The APIs are: %s", identifier, strings.Join(identifiers, ", "))
		}
	}

	scopes, err := resolveScopes(state, in)
	if err != nil {
		return err
	}

	for _, name := range slices.Sorted(maps.Keys(in.Hints)) {
		if !slices.ContainsFunc(state.APIs, func(api API) bool {
			return slices.ContainsFunc(scopes[api.Identifier].All, func(s Scope) bool { return s.Name == name })
		}) {
			return invalidFlag("--hint", "%s is not a scope of the APIs", name)
		}
	}

	want := companyDomainURL(in.CompanyDomain)
	if !containsFold(state.Tenant.CompanyDomains, want) && len(state.Tenant.CompanyDomains) >= maxCompanyDomains {
		return &InputError{
			Reason:  "company_domains_full",
			Message: fmt.Sprintf("The tenant already lists %d company domains, which is the most it can hold, so %s cannot be added.", maxCompanyDomains, want),
			Fix:     "Remove a company domain from the tenant setting personal_agent_protocol_company_domains.",
		}
	}

	return nil
}

// GuestScope is a scope that the guest grant of an API could carry.
type GuestScope struct {
	Name        string
	Description string
	// Hints are the hints of the scope, nil when none are known.
	Hints *config.Hints
}

// GuestOffer is the scopes on offer for the guest grant of an API.
type GuestOffer struct {
	Audience string
	Scopes   []GuestScope
}

// AllGuestGrants tells whether the app already has a guest grant for every
// selected API.
func (s *State) AllGuestGrants() bool {
	return len(s.APIs) > 0 && !slices.ContainsFunc(s.APIs, func(api API) bool { return !s.HasGuestGrant[api.Identifier] })
}

// GuestOffers returns the scopes on offer for the guest grant of each API that
// has no guest grant yet. The hints come from --hint, else from the config file.
func GuestOffers(state *State, in Inputs) ([]GuestOffer, error) {
	scopes, err := resolveScopes(state, in)
	if err != nil {
		return nil, err
	}

	var offers []GuestOffer

	for _, api := range state.APIs {
		if state.HasGuestGrant[api.Identifier] {
			continue
		}

		offer := GuestOffer{Audience: api.Identifier}

		for _, scope := range scopes[api.Identifier].All {
			offered := GuestScope{Name: scope.Name, Description: scope.Description}

			if hints, found := in.Hints[scope.Name]; found {
				offered.Hints = &hints
			} else if state.File != nil {
				offered.Hints = state.File.Scopes[scope.Name].Hints
			}

			offer.Scopes = append(offer.Scopes, offered)
		}

		offers = append(offers, offer)
	}

	return offers, nil
}

// ChooseGuestScopes works out the scopes of the guest grant from the flags: the
// scopes named, which every API must define, or every scope when GuestAllScopes
// is set, in which case it returns nil. It returns ErrGuestScopesRequired when
// the guest grant is asked for, an API still lacks it and neither flag is
// given. It does nothing unless the guest grant is asked for.
func ChooseGuestScopes(state *State, in Inputs) (map[string][]string, error) {
	if !in.GuestAPIAccess || in.GuestAllScopes {
		return nil, nil
	}

	if len(in.GuestScopeNames) == 0 {
		if slices.ContainsFunc(state.APIs, func(api API) bool { return !state.HasGuestGrant[api.Identifier] }) {
			return nil, ErrGuestScopesRequired
		}

		return nil, nil
	}

	scopes, err := resolveScopes(state, in)
	if err != nil {
		return nil, err
	}

	chosen := make(map[string][]string, len(state.APIs))

	for _, api := range state.APIs {
		for _, name := range in.GuestScopeNames {
			if !slices.ContainsFunc(scopes[api.Identifier].All, func(s Scope) bool { return s.Name == name }) {
				return nil, &InputError{
					Reason:  "unknown_scope",
					Message: fmt.Sprintf("--guest-scopes: scope %q is not defined on %s", name, api.Identifier),
					Fix:     "Pass scopes that every selected API defines, or --guest-all-scopes.",
				}
			}
		}

		chosen[api.Identifier] = slices.Clone(in.GuestScopeNames)
	}

	return chosen, nil
}

// guestScopeNames returns the scopes the guest grant of an API carries: the
// chosen ones, else every scope when GuestAllScopes is set, else none.
func guestScopeNames(in Inputs, identifier string, scopes apiScopes) []string {
	if names, chosen := in.GuestScopes[identifier]; chosen {
		return names
	}

	if !in.GuestAllScopes {
		return nil
	}

	names := make([]string, len(scopes.All))
	for i, scope := range scopes.All {
		names[i] = scope.Name
	}

	return names
}

// apiScopes holds the scopes of an API once the --scope descriptions are added.
type apiScopes struct {
	All []Scope
	// Changed lists the scopes that get a description or are added, as the plan
	// shows them: a scope that is added reads "<name> (new)", and one that only
	// gets a description reads "<name> (description)".
	Changed []string
	// Details has the same scopes as Changed, with their descriptions.
	Details []ScopeChange
}

func newScope(name string) string { return name + " (new)" }

func describedScope(name string) string { return name + " (description)" }

// resolveScopes adds the --scope descriptions to the scopes of each API, by API
// identifier. A description only goes to a scope that has none: setup never edits
// one. A scope that no API has is added when there is exactly one API. The
// reserved scope BrowserSignInScope is the exception: every API gets it when it
// lacks it, and a missing description is filled in with the default or the one
// given. It stops when a scope offered to agents is left without a description.
func resolveScopes(state *State, in Inputs) (map[string]apiScopes, error) {
	return resolveScopesOf(state, in, true)
}

// resolveScopesOf is resolveScopes. With addReserved false it leaves
// BrowserSignInScope as the tenant has it, for the commands that describe the
// tenant instead of changing it.
func resolveScopesOf(state *State, in Inputs, addReserved bool) (map[string]apiScopes, error) {
	resolved, undescribed, err := collectScopes(state, in, addReserved)
	if err != nil {
		return nil, err
	}

	if len(undescribed) > 0 {
		return nil, scopeDescriptionRequired(undescribed)
	}

	return resolved, nil
}

// collectScopes does the work of resolveScopesOf and returns the scopes left
// without a description instead of stopping for them.
func collectScopes(state *State, in Inputs, addReserved bool) (map[string]apiScopes, []undescribedScope, error) {
	resolved := make(map[string]apiScopes, len(state.APIs))
	seen := map[string]bool{}

	var undescribed []undescribedScope

	for _, api := range state.APIs {
		var res apiScopes

		for _, scope := range api.Scopes {
			if description, given := in.Scopes[scope.Name]; given {
				seen[scope.Name] = true

				switch {
				case scope.Description == description:
				case scope.Description != "":
					return nil, nil, &InputError{
						Reason:  "scope_description_exists",
						Message: fmt.Sprintf("Scope %s of %s already has a description, and setup never edits one.", scope.Name, api.Identifier),
						Fix:     fmt.Sprintf("Remove --scope %s, or change the description of the scope in the Auth0 Dashboard.", scope.Name),
					}
				default:
					scope.Description = description
					res.Changed = append(res.Changed, describedScope(scope.Name))
					res.Details = append(res.Details, ScopeChange{Name: scope.Name, Description: description})
				}
			}

			if addReserved && scope.Description == "" && scope.Name == BrowserSignInScope {
				scope.Description = BrowserSignInDescription
				res.Changed = append(res.Changed, describedScope(scope.Name))
				res.Details = append(res.Details, ScopeChange{Name: scope.Name, Description: BrowserSignInDescription})
			}

			if scope.Description == "" {
				undescribed = append(undescribed, undescribedScope{name: scope.Name, api: api.Identifier})
			}

			res.All = append(res.All, scope)
		}

		if addReserved && !slices.ContainsFunc(api.Scopes, func(s Scope) bool { return s.Name == BrowserSignInScope }) {
			seen[BrowserSignInScope] = true

			description := cmp.Or(in.Scopes[BrowserSignInScope], BrowserSignInDescription)
			res.All = append(res.All, Scope{Name: BrowserSignInScope, Description: description})
			res.Changed = append(res.Changed, newScope(BrowserSignInScope))
			res.Details = append(res.Details, ScopeChange{Name: BrowserSignInScope, Description: description, New: true})
		}

		resolved[api.Identifier] = res
	}

	for _, name := range slices.Sorted(maps.Keys(in.Scopes)) {
		if seen[name] {
			continue
		}

		if len(state.APIs) != 1 {
			return nil, nil, &InputError{
				Reason:  "scope_not_found",
				Message: fmt.Sprintf("Scope %s is not a scope of any of the APIs, and with several APIs there is no way to tell which one gets it.", name),
				Fix:     "Add the scope to its API first.",
			}
		}

		identifier := state.APIs[0].Identifier
		res := resolved[identifier]
		res.All = append(res.All, Scope{Name: name, Description: in.Scopes[name]})
		res.Changed = append(res.Changed, newScope(name))
		res.Details = append(res.Details, ScopeChange{Name: name, Description: in.Scopes[name], New: true})
		resolved[identifier] = res
	}

	return resolved, undescribed, nil
}

// undescribedScope is a scope of an API that has no description.
type undescribedScope struct {
	name string
	api  string
}

// scopeDescriptionRequired builds the error for scopes that agents would be
// offered without a description. The FIX line is a ready-to-paste --scope flag
// for each missing scope, up to maxListedScopeFixes of them.
func scopeDescriptionRequired(undescribed []undescribedScope) *InputError {
	labels := make([]string, len(undescribed))
	for i, scope := range undescribed {
		labels[i] = scope.name + " (" + scope.api + ")"
	}

	var names []string

	for _, scope := range undescribed {
		if !slices.Contains(names, scope.name) {
			names = append(names, scope.name)
		}
	}

	flag := func(name string) string { return "--scope '" + name + "=<what this lets an agent do>'" }

	fix := "Pass " + flag(names[0]) + "."

	if len(names) > 1 {
		var b strings.Builder

		b.WriteString("Pass --scope for each of them:")

		for i, name := range names {
			if i == maxListedScopeFixes {
				fmt.Fprintf(&b, "\n         and %d more", len(names)-i)

				break
			}

			b.WriteString("\n         " + flag(name))
		}

		fix = b.String()
	}

	return &InputError{
		Reason:  ReasonScopeDescriptionRequired,
		Message: "Every scope offered to agents needs a description. These have none: " + strings.Join(labels, ", ") + ".",
		Fix:     fix,
	}
}

// ScopeQuestions returns a question for each scope that is offered to agents and
// has no description, once the --scope descriptions are added. A scope on
// several APIs is asked about once, as one --scope description covers them all.
// It returns nothing when no scope lacks one, or when the scopes cannot be
// resolved for another reason, which Check reports.
func ScopeQuestions(state *State, in Inputs) []Question {
	_, undescribed, err := collectScopes(state, in, true)
	if err != nil {
		return nil
	}

	var names []string
	apis := map[string][]string{}

	for _, scope := range undescribed {
		if !slices.Contains(names, scope.name) {
			names = append(names, scope.name)
		}

		apis[scope.name] = append(apis[scope.name], scope.api)
	}

	var questions []Question

	for _, name := range names {
		questions = append(questions, Question{
			Flag:   "--scope '" + name + "=<description>'",
			Title:  "Description of " + clean(name),
			Reason: fmt.Sprintf("Agents show it to the user before asking for approval. It is a scope of %s.", clean(strings.Join(apis[name], ", "))),
			Prompt: "Text",
			apply: func(in *Inputs, value string) error {
				value, err := checkDescription("--scope", value)
				if err != nil {
					return err
				}

				if in.Scopes == nil {
					in.Scopes = map[string]string{}
				}
				in.Scopes[name] = value

				return nil
			},
		})
	}

	return questions
}

// HintScope is a scope that agents are offered and that has no hints yet.
type HintScope struct {
	Name        string
	Description string
	// APIs are the identifiers of the APIs that have the scope.
	APIs []string
}

// HintScopes returns the scopes of the APIs that have no hints, in the order of
// the APIs. A scope has hints when --hint gave it some or the existing config file
// has them. The reserved BrowserSignInScope never needs them: it has a default.
// With addReserved false the scopes are the ones the tenant has, as for a command
// that does not change it. It returns nothing when the scopes cannot be resolved,
// which the command reports in its own way.
func HintScopes(state *State, in Inputs, addReserved bool) []HintScope {
	scopes, err := resolveScopesOf(state, in, addReserved)
	if err != nil {
		return nil
	}

	var found []HintScope

	for _, api := range state.APIs {
		for _, scope := range scopes[api.Identifier].All {
			if scope.Name == BrowserSignInScope || hasHints(state, in, scope.Name) {
				continue
			}

			if i := slices.IndexFunc(found, func(h HintScope) bool { return h.Name == scope.Name }); i >= 0 {
				found[i].APIs = append(found[i].APIs, api.Identifier)

				continue
			}

			found = append(found, HintScope{Name: scope.Name, Description: scope.Description, APIs: []string{api.Identifier}})
		}
	}

	return found
}

func hasHints(state *State, in Inputs, scope string) bool {
	if _, found := in.Hints[scope]; found {
		return true
	}

	return state.File != nil && state.File.Scopes[scope].Hints != nil
}

// companyDomainURL returns the form of a company domain that the tenant setting
// holds.
func companyDomainURL(domain string) string {
	return "https://" + domain
}

func containsFold(list []string, value string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, value) })
}
