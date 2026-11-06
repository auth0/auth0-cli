package setup

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// NotReadyError lists the Poppy settings the tenant lacks, for a command that
// must not change the tenant.
type NotReadyError struct {
	Problems []string
	Fix      string
}

// Error lists the problems, then the FIX line.
func (e *NotReadyError) Error() string {
	return "the tenant is not set up for Poppy:\n  - " + strings.Join(e.Problems, "\n  - ") + "\n  FIX  " + e.Fix
}

// CompanyDomain returns the company domain the config file is for: the one given,
// else the one in the existing file, else the only one the tenant lists. When
// the tenant lists several and none is picked, it returns the choices. When it
// lists none, it returns neither.
func (s *State) CompanyDomain(in Inputs) (domain string, choices []string) {
	if in.CompanyDomain != "" {
		return in.CompanyDomain, nil
	}

	if s.File != nil && s.File.Organization.Domain != "" {
		return s.File.Organization.Domain, nil
	}

	hosts := make([]string, len(s.Tenant.CompanyDomains))
	for i, entry := range s.Tenant.CompanyDomains {
		hosts[i] = strings.ToLower(strings.TrimPrefix(entry, "https://"))
	}

	if len(hosts) == 1 {
		return hosts[0], nil
	}

	return "", hosts
}

// Ready checks that the tenant is set up for Poppy, for the settings that the
// config file depends on: the company domain is listed, each API has Poppy on and
// the browser join is on. It returns a *NotReadyError naming every setting that
// is not.
func Ready(state *State, in Inputs) error {
	var problems []string

	switch want := companyDomainURL(in.CompanyDomain); {
	case in.CompanyDomain == "":
		problems = append(problems, "tenant setting personal_agent_protocol_company_domains is empty")
	case !containsFold(state.Tenant.CompanyDomains, want):
		problems = append(problems, fmt.Sprintf("tenant setting personal_agent_protocol_company_domains does not list %s", want))
	}

	for _, api := range state.APIs {
		if !api.Poppy {
			problems = append(problems, fmt.Sprintf("API %s: use_for_personal_agent_protocol is off", api.Identifier))
		}
	}

	if !state.App.BrowserJoin {
		problems = append(problems, fmt.Sprintf("app %s: allow_personal_agent_browser_join is off", state.App.Name))
	}

	if len(problems) == 0 {
		return nil
	}

	return &NotReadyError{Problems: problems, Fix: "Run: auth0 poppy setup --company-domain " + cmp.Or(in.CompanyDomain, "<host>")}
}

// CheckIssuer returns the error that Derive and Build return when the existing
// config file belongs to another issuer than the tenant, unless the inputs allow
// overwriting it. It reads only the state, so a command can refuse before it asks
// any question.
func CheckIssuer(state *State, in Inputs) error {
	return config.CheckIssuer(state.File, state.Issuer(), config.MergeOptions{ForceConfig: in.ForceConfig})
}

// Derive builds the content of the config file from the state and the inputs, as
// the file will read once setup has made its changes. Values Auth0 does not
// store come from the inputs, else from the existing file; when neither has one,
// it returns a *config.MissingFieldsError. Unlike Build, it adds no scope the
// tenant lacks.
func Derive(state *State, in Inputs) (*config.Config, error) {
	// This describes the tenant as it is, so it does not add BrowserSignInScope:
	// only setup does that.
	scopes, err := resolveScopesOf(state, in, false)
	if err != nil {
		return nil, err
	}

	return derive(state, in, scopes)
}

// defaultHints returns the hints a scope gets when neither the inputs nor the
// config file have any. Only the browser sign-in scope has a default: it adds no
// access of its own, so it neither changes anything nor carries personal data.
func defaultHints(scope string) *config.Hints {
	if scope != BrowserSignInScope {
		return nil
	}

	return &config.Hints{Changes: "none"}
}

func derive(state *State, in Inputs, scopes map[string]apiScopes) (*config.Config, error) {
	derived := &config.Config{
		Organization: config.Organization{Name: state.Tenant.FriendlyName},
		Issuer:       state.Issuer(),
		Scopes:       map[string]config.Scope{},
	}

	given := make(map[string]config.API, len(state.APIs))
	descriptions := make(map[string]string, len(state.APIs))

	for _, api := range state.APIs {
		given[api.Identifier] = config.API{URL: in.APIURLs[api.Identifier]}
		descriptions[api.Identifier] = api.Name

		for _, scope := range scopes[api.Identifier].All {
			if _, found := derived.Scopes[scope.Name]; !found {
				derived.Scopes[scope.Name] = config.Scope{Description: scope.Description, Hints: defaultHints(scope.Name)}
			}
		}
	}

	cfg, err := config.Merge(state.File, derived, config.Inputs{
		OrganizationName: in.Name,
		CompanyDomain:    in.CompanyDomain,
		ClientID:         state.App.ClientID,
		APIs:             given,
		APIDescriptions:  descriptions,
		Hints:            in.Hints,
	}, config.MergeOptions{ForceConfig: in.ForceConfig})

	var ambiguous *config.AmbiguousAPIError
	if errors.As(err, &ambiguous) {
		return nil, &InputError{
			Reason: "ambiguous_api_entry",
			Message: fmt.Sprintf("The config file lists %d APIs at the origin of %s: %s.",
				len(ambiguous.URLs), ambiguous.Identifier, strings.Join(ambiguous.URLs, ", ")),
			Fix: "Pass --api-url " + ambiguous.Identifier + "=<url> with the URL of the one that is this API.",
		}
	}

	return cfg, err
}
