package setup

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// maxCompanyDomains is the number of company domains a tenant can list.
const maxCompanyDomains = 5

// Check validates the inputs against the state. It stops with an *InputError and
// a FIX line when the browser join cannot work or the scopes are not ready to be
// offered to agents.
func Check(state *State, in Inputs) error {
	if !in.NoWeb && !state.App.FirstParty {
		return &InputError{
			Reason:  ReasonAppNotFirstParty,
			Message: fmt.Sprintf("The browser join needs a first-party web app. %s is not first-party.", state.App.Name),
			Fix:     "Pass a first-party web app with --app.",
		}
	}

	return CheckAPIs(state, in)
}

// CheckAPIs is Check for the part of the state that does not need the web app:
// the APIs, their scopes and hints, and the company domains of the tenant. A
// command that asks about the APIs before it picks the web app uses it, as the
// state has no app yet.
func CheckAPIs(state *State, in Inputs) error {
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

	want := in.CompanyDomain
	if !HasCompanyDomain(state.Tenant.CompanyDomains, want) && len(state.Tenant.CompanyDomains) >= maxCompanyDomains {
		return &InputError{
			Reason:  ReasonCompanyDomainsFull,
			Message: fmt.Sprintf("The tenant already lists %d company domains, which is the most it can hold, so %s cannot be added.", maxCompanyDomains, want),
			Fix:     "Remove a company domain from the tenant setting personal_agent_protocol_company_domains.",
		}
	}

	return nil
}

// NormalizeCompanyDomain returns the bare lower-case host of an entry of the
// tenant setting personal_agent_protocol_company_domains. The setting holds bare
// hosts such as shop.example. An https://shop.example origin, which an older
// build or a person may have written, is read as the host it names.
func NormalizeCompanyDomain(entry string) string {
	if len(entry) >= len("https://") && strings.EqualFold(entry[:len("https://")], "https://") {
		entry = strings.TrimSuffix(entry[len("https://"):], "/")
	}

	return strings.ToLower(entry)
}

// IsBareCompanyDomain reports whether an entry of the tenant setting is a bare
// lower-case host: no scheme, port, path, query or trailing slash. It is the
// form the Management API accepts.
func IsBareCompanyDomain(entry string) bool {
	return entry != "" && entry == strings.ToLower(entry) && !strings.ContainsAny(entry, "/:@?#\\ \t")
}

// HasCompanyDomain tells whether the entries list the host. Entries are compared
// as hosts, so an https://shop.example origin matches shop.example.
func HasCompanyDomain(entries []string, host string) bool {
	want := NormalizeCompanyDomain(host)

	return slices.ContainsFunc(entries, func(entry string) bool { return NormalizeCompanyDomain(entry) == want })
}

// companyDomainsWith returns the list the tenant setting holds once the host is
// added: every existing entry as a bare host, without repeats, then the host.
func companyDomainsWith(entries []string, host string) []string {
	list := make([]string, 0, len(entries)+1)
	for _, entry := range append(slices.Clone(entries), host) {
		if bare := NormalizeCompanyDomain(entry); !slices.Contains(list, bare) {
			list = append(list, bare)
		}
	}

	return list
}
