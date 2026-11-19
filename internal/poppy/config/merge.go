package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// defaultAPIType is the API type used when none is given anywhere.
const defaultAPIType = "openapi"

// ErrIssuerMismatch is returned by Merge when the existing file belongs to a
// different issuer than the tenant, and the overwrite was not forced.
var ErrIssuerMismatch = errors.New("the existing config file belongs to a different issuer")

// MissingFieldsError lists the values that Auth0 does not store and that were
// given neither in the existing file nor as inputs.
type MissingFieldsError struct {
	// Fields holds config paths such as organization.domain, sorted.
	Fields []string
}

// Error implements the error interface.
func (e *MissingFieldsError) Error() string {
	return "missing required values: " + strings.Join(e.Fields, ", ")
}

// AmbiguousAPIError is returned by Merge when several entries of apis are at the
// origin of an API and a value was given for the API that only one of them can
// take, so that the file does not say which one it is.
type AmbiguousAPIError struct {
	Identifier string
	// URLs are the URLs of the entries, in file order.
	URLs []string
}

// Error implements the error interface.
func (e *AmbiguousAPIError) Error() string {
	return fmt.Sprintf("the config file lists %d APIs at the origin of %s: %s", len(e.URLs), e.Identifier, strings.Join(e.URLs, ", "))
}

// Inputs holds values given by flags or prompts. A zero value means the value
// was not given.
type Inputs struct {
	OrganizationName string
	// CompanyDomain is a bare host such as shop.example.
	CompanyDomain string
	// ClientID is the client ID of the web app, which the browser join address
	// holds.
	ClientID string
	// APIs holds the APIs of the tenant that the file describes, keyed by API
	// identifier, with the values that Auth0 does not store. An API with no
	// value is still listed, so that its entry is found in the existing file.
	APIs map[string]API
	// APIDescriptions holds, by API identifier, the description to use when
	// neither APIs nor the existing entry has one. The CLI passes the name of the
	// API in the tenant.
	APIDescriptions map[string]string
	// Hints holds scope hints by scope name.
	Hints map[string]Hints
	// WebUnscoped is the owner's answer for web.unscoped. Nil means it was not
	// given, and the existing file's value is kept.
	WebUnscoped *bool
	// AuthDisabled sets auth.disabled: the owner turned the API part off.
	AuthDisabled bool
	// WebDisabled sets web.disabled and drops the browser join address and
	// web.unscoped: the owner turned the web part off.
	WebDisabled bool
}

// MergeOptions changes how Merge behaves.
type MergeOptions struct {
	// ForceConfig allows overwriting a file that belongs to another issuer.
	ForceConfig bool
}

// CheckIssuer returns an error that wraps ErrIssuerMismatch when the existing file
// (which may be nil) has an issuer other than the given one, unless the options
// allow overwriting it. Merge makes the same check.
func CheckIssuer(existing *Config, issuer string, opts MergeOptions) error {
	if existing != nil && existing.Issuer != "" && existing.Issuer != issuer && !opts.ForceConfig {
		return fmt.Errorf("%w: the file has %q and the tenant has %q", ErrIssuerMismatch, existing.Issuer, issuer)
	}

	return nil
}

// Merge builds the config to write from the values derived from the tenant,
// the existing file (which may be nil) and the inputs. It does not modify its
// arguments.
//
// The result is always at the current protocol version, so a file written at an
// older supported version is upgraded. The web.unscoped, web.disabled and
// auth.disabled settings of the existing file are kept, because only the website
// owner knows them, except that an input for web.unscoped (the owner's answer)
// overrides the file's value, the inputs AuthDisabled and WebDisabled turn the
// settings on, and a part that is on for the run (APIs given with AuthDisabled
// unset, ClientID given with WebDisabled unset) clears the file's disabled setting
// for it. Turning the web part off also removes the browser join address and
// web.unscoped. So is every member of the file that the CLI does not use, such as agent,
// feedback or extensions.
//
// Tenant-derived values always win over the existing file. For values Auth0
// does not store, an input wins over the existing file, and the existing file
// over the hints of the derived scopes. A scope of the existing file that the
// tenant values do not derive stays as it is, with its description and hints,
// because the derived scopes are only those of the APIs of this run and the file
// may describe more. The entry of apis for an API of the tenant is found by the
// URL given, else by the origin of the API identifier; it is updated where it
// is, and an API with no entry gets a new one at the end. When several entries
// are at the origin (the same API published as openapi and as mcp, say), all of
// them are updated, and only the description fill applies to them. Entries of
// other APIs stay as they are. If a required value is in none of these places,
// Merge returns a *MissingFieldsError.
func Merge(existing, derived *Config, in Inputs, opts MergeOptions) (*Config, error) {
	if derived == nil {
		return nil, errors.New("the values derived from the tenant are required")
	}

	if err := CheckIssuer(existing, derived.Issuer, opts); err != nil {
		return nil, err
	}
	if existing == nil {
		existing = &Config{}
	}

	out := &Config{
		ProtocolVersion: ProtocolVersion,
		Issuer:          derived.Issuer,
		Organization: Organization{
			Name:    cmp.Or(in.OrganizationName, derived.Organization.Name, existing.Organization.Name),
			Domain:  cmp.Or(in.CompanyDomain, existing.Organization.Domain),
			LogoURI: cmp.Or(derived.Organization.LogoURI, existing.Organization.LogoURI),
			extra:   maps.Clone(existing.Organization.extra),
		},
		Auth:   Auth{Disabled: existing.Auth.Disabled || in.AuthDisabled, DisabledSet: existing.Auth.DisabledSet || in.AuthDisabled},
		Scopes: make(map[string]Scope, len(derived.Scopes)+len(existing.Scopes)),
		APIs:   make([]API, len(existing.APIs)),
		Web: Web{
			BrowserSessionEndpoint: existing.Web.BrowserSessionEndpoint,
			Unscoped:               existing.Web.Unscoped,
			UnscopedSet:            existing.Web.UnscopedSet,
			Disabled:               existing.Web.Disabled,
		},
		extra: existing.extra.clone(),
	}

	for i, api := range existing.APIs {
		api.extra = maps.Clone(api.extra)
		out.APIs[i] = api
	}

	var missing []string
	if out.Organization.Name == "" {
		missing = append(missing, "organization.name")
	}
	if out.Organization.Domain == "" {
		missing = append(missing, "organization.domain")
	}

	if out.Issuer != "" && in.ClientID != "" {
		out.Web.BrowserSessionEndpoint = BrowserSessionEndpoint(out.Issuer, in.ClientID)
	}

	// A part that is on for this run is no longer off in the file.
	if in.ClientID != "" && !in.WebDisabled {
		out.Web.Disabled = false
	}

	// An explicit false in the file is kept as the file has it.
	if len(in.APIs) > 0 && !in.AuthDisabled && out.Auth.Disabled {
		out.Auth = Auth{}
	}

	if in.WebUnscoped != nil {
		out.Web.Unscoped, out.Web.UnscopedSet = *in.WebUnscoped, true
	}

	if in.WebDisabled {
		out.Web = Web{Disabled: true}
	}

	for name, scope := range existing.Scopes {
		if scope.Hints != nil {
			h := *scope.Hints
			h.extra = maps.Clone(h.extra)
			scope.Hints = &h
		}

		out.Scopes[name] = scope
	}

	for name, scope := range derived.Scopes {
		merged := Scope{Description: scope.Description}
		if h, ok := in.Hints[name]; ok {
			if old := existing.Scopes[name]; old.Hints != nil {
				h.extra = maps.Clone(old.Hints.extra)
			}

			merged.Hints = &h
		} else if old := existing.Scopes[name]; old.Hints != nil {
			h := *old.Hints
			h.extra = maps.Clone(h.extra)
			merged.Hints = &h
		} else if scope.Hints != nil {
			h := *scope.Hints
			merged.Hints = &h
		}
		out.Scopes[name] = merged
	}

	claimed := map[int]bool{}

	for _, id := range slices.Sorted(maps.Keys(in.APIs)) {
		given := in.APIs[id]

		entries, err := entriesOf(existing, id, given, claimed)
		if err != nil {
			return nil, err
		}

		if len(entries) == 0 {
			if given.URL == "" {
				missing = append(missing, "apis["+id+"].url")
				continue
			}

			out.APIs = append(out.APIs, API{
				Type:        cmp.Or(given.Type, defaultAPIType),
				URL:         given.URL,
				Description: cmp.Or(given.Description, in.APIDescriptions[id]),
			})

			if out.APIs[len(out.APIs)-1].Description == "" {
				missing = append(missing, "apis["+id+"].description")
			}

			continue
		}

		for _, i := range entries {
			claimed[i] = true
			old := out.APIs[i]
			out.APIs[i] = API{
				Type:        cmp.Or(given.Type, old.Type, defaultAPIType),
				URL:         cmp.Or(given.URL, old.URL),
				Description: cmp.Or(given.Description, old.Description, in.APIDescriptions[id]),
				extra:       old.extra,
			}

			if out.APIs[i].Description == "" {
				missing = append(missing, "apis["+id+"].description")
			}
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &MissingFieldsError{Fields: missing}
	}

	return out, nil
}

// entriesOf finds the indexes of the entries of apis for an API of the tenant,
// or none. An entry with the URL given is the one. Else the entries at the origin
// of the identifier that no other API has taken are, and when there are several
// they are all updated, since the same API may be published in several forms.
// With several entries, a value given for the API (URL, type or description)
// cannot say which entry it is for, so that case is an *AmbiguousAPIError.
func entriesOf(existing *Config, identifier string, given API, claimed map[int]bool) ([]int, error) {
	if given.URL != "" {
		if i := slices.IndexFunc(existing.APIs, func(a API) bool { return a.URL == given.URL }); i >= 0 {
			return []int{i}, nil
		}
	}

	var candidates []int
	for _, i := range existing.EntriesFor(identifier) {
		if !claimed[i] {
			candidates = append(candidates, i)
		}
	}

	if len(candidates) < 2 || (given.URL == "" && given.Type == "" && given.Description == "") {
		return candidates, nil
	}

	urls := make([]string, len(candidates))
	for j, i := range candidates {
		urls[j] = existing.APIs[i].URL
	}

	return nil, &AmbiguousAPIError{Identifier: identifier, URLs: urls}
}
