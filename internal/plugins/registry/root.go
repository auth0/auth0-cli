package registry

import (
	"embed"
)

// trustedFS holds the TUF root that ships with the CLI. The directory also holds
// a README, so that it exists in a build that embeds no root yet.
//
// TODO(DXCDT-2611): Before the first release that ships the plugin framework,
// add the production root as trusted/root.json. It is the version 1 root of the
// registry: 3 root keys with a threshold of 2, and one key each for the targets,
// snapshot and timestamp roles, with consistent snapshots. Document the key
// ceremony in the release process, and check that the root is present and
// verifies in a release build. Until then every verification fails with
// ErrNoTrustedKeys, which is the safe default.
//
//go:embed trusted
var trustedFS embed.FS

// trustedRootPath is the path of the root in trustedFS.
const trustedRootPath = "trusted/root.json"

// productionRoot returns the root that is embedded in the CLI, or nil when the
// build embeds none.
func productionRoot() []byte {
	data, err := trustedFS.ReadFile(trustedRootPath)
	if err != nil {
		return nil
	}

	return data
}

// trustedRoot returns the root that the client starts its chain of trust from,
// which is Root when it is set and the embedded root otherwise. It is empty when
// there is none.
func (c *Client) trustedRoot() []byte {
	if c.Root != nil {
		return c.Root
	}

	return embeddedRoot()
}
