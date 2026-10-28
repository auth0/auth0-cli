//go:build !plugintest

package store

// allowLoopback is false in every build except one made with the plugintest
// tag, so a plugin can only be downloaded over https from the release hosts.
const allowLoopback = false
