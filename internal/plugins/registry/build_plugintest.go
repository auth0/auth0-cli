//go:build plugintest

package registry

import (
	_ "embed"
	"os"
)

// allowLoopbackHTTP lets a build made with the plugintest tag fetch a registry
// or a plugin file over plain http from the loopback address, so that a
// registry can be served from the same machine without a certificate.
const allowLoopbackHTTP = true

// testRootEnv names the environment variable that holds the path of a root.json
// that a build made with the plugintest tag trusts instead of the embedded test
// root. It exists so that tests that run the CLI as a subprocess can trust a
// registry they publish, without the CLI having any private key. A regular
// build does not read it.
const testRootEnv = "AUTH0_CLI_PLUGIN_TEST_ROOT"

// testRoot is the root that a build made with the plugintest tag embeds. It has
// public keys only, and the private halves were thrown away when it was made, so
// nothing can be published that this root would accept. Tests publish with a
// registrytest.Repo of their own, and hand its root to the client.
//
//go:embed testdata/test-root.json
var testRoot []byte

// embeddedRoot returns the root that this build of the CLI trusts, which is the
// one in the file that testRootEnv names when it is set, and the embedded test
// root otherwise. A file that cannot be read gives no root, so the client fails
// closed instead of trusting the embedded one.
func embeddedRoot() []byte {
	if path := os.Getenv(testRootEnv); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		return data
	}

	return testRoot
}
