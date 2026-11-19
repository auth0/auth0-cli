package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Violation is one problem found by Validate.
type Violation struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	// Fix says what to do about it. It can be empty.
	Fix string `json:"fix,omitempty"`
}

// ValidationError holds the violations of an invalid config.
type ValidationError struct {
	// File is the config file path, when known.
	File       string
	Violations []Violation
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		part := v.Path + " " + v.Message
		if v.Fix != "" {
			part += "\n  FIX  " + v.Fix
		}

		parts = append(parts, part)
	}

	subject := "config"
	if e.File != "" {
		subject = e.File
	}

	return fmt.Sprintf("%s is invalid: %s", subject, strings.Join(parts, "; "))
}

var hostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

// maxHostLength is the longest host name, in characters.
const maxHostLength = 253

// ValidHost tells whether s is a lowercase host name such as shop.example: dot
// separated labels of 63 characters at most, in letters, digits and hyphens, and
// 253 characters at most in all.
func ValidHost(s string) bool {
	return len(s) <= maxHostLength && hostPattern.MatchString(s)
}

// NormalizeDomain turns shop.example or https://shop.example/ into the bare,
// lower-case host. Paths, ports and other schemes are rejected.
func NormalizeDomain(s string) (string, error) {
	host := strings.TrimSpace(s)

	if scheme, rest, found := strings.Cut(host, "://"); found {
		if !strings.EqualFold(scheme, "https") {
			return "", fmt.Errorf("the domain %q must be a host name such as shop.example, not a %s URL", s, scheme)
		}
		host = strings.TrimSuffix(rest, "/")
	}

	switch {
	case host == "":
		return "", errors.New("the domain is empty")
	case strings.ContainsAny(host, "/?#@"):
		return "", fmt.Errorf("the domain %q must not include a path, query or user: use a host name such as shop.example", s)
	case strings.Contains(host, ":"):
		return "", fmt.Errorf("the domain %q must not include a port: use a host name such as shop.example", s)
	}

	host = strings.ToLower(host)
	if !ValidHost(host) {
		return "", fmt.Errorf("the domain %q is not a valid host name such as shop.example", s)
	}

	return host, nil
}

// Validate checks the config against the rules of the protocol for poppy.json,
// for the members the file has. It returns the violations in a stable order,
// member by member, and nil when the config is valid. Paths are the paths of
// the file.
func (c *Config) Validate() []Violation {
	var out []Violation
	add := func(path, msg string) {
		out = append(out, Violation{Path: path, Message: msg})
	}
	check := func(path string, err error) {
		if err != nil {
			add(path, err.Error())
		}
	}

	switch {
	case Understands(c.ProtocolVersion):
	case c.ProtocolVersion == "":
		add("protocol_version", "is required")
	default:
		add("protocol_version", fmt.Sprintf(`must be one of "0.3", "0.4" or 1.x, got %q`, c.ProtocolVersion))
	}

	if strings.TrimSpace(c.Organization.Name) == "" {
		add("organization.name", "is required")
	}

	check("organization.domain", checkDomain(c.Organization.Domain))

	if c.Organization.LogoURI != "" {
		check("organization.logo_uri", CheckHTTPS(c.Organization.LogoURI))
	}

	if c.Issuer != "" {
		check("auth.issuer", checkIssuer(c.Issuer))
	}

	for _, name := range slices.Sorted(maps.Keys(c.Scopes)) {
		if strings.TrimSpace(c.Scopes[name].Description) == "" {
			add("auth.scopes."+name, "must not be empty")
		}

		if hints := c.Scopes[name].Hints; hints != nil && !slices.Contains(ValidChanges, hints.Changes) {
			add("auth.scope_hints."+name+".changes", fmt.Sprintf("must be one of %s, got %q", strings.Join(ValidChanges, ", "), hints.Changes))
		}
	}

	for i, api := range c.APIs {
		prefix := "apis[" + strconv.Itoa(i) + "]"

		// The list of types is open (protocol section 7), so any type is accepted.
		if strings.TrimSpace(api.Type) == "" {
			add(prefix+".type", "is required")
		}

		check(prefix+".url", CheckAPIURL(api.URL))

		if strings.TrimSpace(api.Description) == "" {
			add(prefix+".description", "is required")
		}
	}

	if c.Web.BrowserSessionEndpoint != "" {
		check("web.browser_session_endpoint", CheckHTTPS(c.Web.BrowserSessionEndpoint))
	}

	if !c.hasAgentAPIsOrWeb() {
		out = append(out, Violation{
			Path:    rootPath,
			Message: "needs at least one of agent, apis or web",
			Fix:     "Run auth0 poppy setup --api <identifier> to add an apis entry, or add an agent or web member to the file.",
		})
	}

	return append(out, c.problems...)
}

// hasAgentAPIsOrWeb tells whether the document has at least one of agent, apis
// and web, as the protocol requires.
func (c *Config) hasAgentAPIsOrWeb() bool {
	_, agent := c.extra.top["agent"]

	return agent || len(c.APIs) > 0 || c.extra.emptyWeb || c.Web != (Web{})
}

func checkDomain(d string) error {
	switch {
	case d == "":
		return errors.New("is required")
	case !ValidHost(d):
		return errors.New("must be a lowercase host name such as shop.example")
	}
	return nil
}

func checkIssuer(s string) error {
	if s == "" {
		return errors.New("is required")
	}

	u, err := parseHTTPS(s)
	switch {
	case err != nil:
		return err
	case u.Path == "":
		return errors.New("must end with a trailing slash")
	case u.Path != "/":
		return errors.New("must not have a path other than /")
	}
	return nil
}

// CheckHTTPS accepts any https URL with a host. Unlike the other URLs it may
// carry a query, such as a cache-busting version.
func CheckHTTPS(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("must be an https URL")
	}

	return nil
}

// CheckAPIURL checks the URL of an API: it is required, must be an https URL and
// must not carry credentials: the origin of the URL is what links it to an API of the
// tenant.
func CheckAPIURL(s string) error {
	if err := checkHTTPSRequired(s); err != nil {
		return err
	}

	if u, err := url.Parse(s); err == nil && u.User != nil {
		return errors.New("must not contain credentials")
	}

	return nil
}

// checkHTTPSRequired is CheckHTTPS for a value that must be present.
func checkHTTPSRequired(s string) error {
	if s == "" {
		return errors.New("is required")
	}

	return CheckHTTPS(s)
}

// parseHTTPS parses s and checks that it is an https URL with a host, no
// credentials, no query and no fragment.
func parseHTTPS(s string) (*url.URL, error) {
	if !strings.HasPrefix(s, "https://") {
		return nil, errors.New("must be an https URL")
	}

	u, err := url.Parse(s)
	switch {
	case err != nil:
		return nil, errors.New("is not a valid URL")
	case u.Host == "":
		return nil, errors.New("must include a host")
	case u.User != nil || strings.ContainsAny(s, "?#"):
		return nil, errors.New("must not contain credentials, a query or a fragment")
	}
	return u, nil
}
