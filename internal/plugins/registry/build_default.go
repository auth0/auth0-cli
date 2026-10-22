//go:build !plugintest

package registry

// allowLoopbackHTTP is false in every build except one made with the plugintest
// tag, so a registry or a plugin file can only be fetched over https.
const allowLoopbackHTTP = false

// testKeys returns no key, because only a build made with the plugintest tag
// trusts a test key.
func testKeys() KeySet {
	return nil
}
