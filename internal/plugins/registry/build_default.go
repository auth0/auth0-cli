//go:build !plugintest

package registry

// allowLoopbackHTTP is false in every build except one made with the plugintest
// tag, so a registry or a plugin file can only be fetched over https.
const allowLoopbackHTTP = false

// embeddedRoot returns the root that this build of the CLI trusts, which is the
// production root and nothing else.
func embeddedRoot() []byte {
	return productionRoot()
}
