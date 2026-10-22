package registry

import (
	"fmt"
	"net/url"
	"slices"

	"github.com/auth0/auth0-cli/internal/config"
)

// The built-in registry that every CLI knows about.
const (
	DefaultRegistryName = "auth0"
	DefaultRegistryURL  = "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/registry/index.json"
)

// Registry is a plugin registry.
type Registry struct {
	Name string

	// URL is the address of the index.json of the registry.
	URL string
}

// ResolveRegistries returns the registries to use, given the registries from
// the config file.
//
// The built-in auth0 registry is always there. A configured registry named
// auth0 can only change its URL, to point at a mirror, and the mirror is still
// verified with the keys embedded in the CLI. A registry with any other name is
// rejected, because custom registries are not supported yet.
func ResolveRegistries(configured []config.RegistryConfig) ([]Registry, error) {
	builtin := Registry{Name: DefaultRegistryName, URL: DefaultRegistryURL}

	for n, entry := range configured {
		if entry.Name != DefaultRegistryName {
			return nil, fmt.Errorf("registries[%d]: cannot use registry %q, custom registries are not supported yet", n, entry.Name)
		}

		if n > 0 {
			return nil, fmt.Errorf("registries[%d]: registry %q is configured more than once", n, entry.Name)
		}

		if err := checkURL(entry.URL); err != nil {
			return nil, fmt.Errorf("registries[%d].url: %w", n, err)
		}

		builtin.URL = entry.URL
	}

	return []Registry{builtin}, nil
}

// checkURL reports an error that wraps ErrInsecureURL unless raw is an https
// URL. A build made with the plugintest tag also accepts http for the loopback
// addresses 127.0.0.1, localhost and [::1].
func checkURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%q is not a valid URL: %w", raw, err)
	}

	if parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%w: %q needs a host and no user name or password", ErrInsecureURL, raw)
	}

	if parsed.Scheme == "https" {
		return nil
	}

	if parsed.Scheme == "http" && allowLoopbackHTTP && slices.Contains([]string{"127.0.0.1", "localhost", "::1"}, parsed.Hostname()) {
		return nil
	}

	return fmt.Errorf("%w: %q", ErrInsecureURL, raw)
}
