//go:build plugintest

package store

// allowLoopback lets a build made with the plugintest tag download a plugin
// from the loopback address, over http or https, so that a registry and its
// artifacts can be served from the same machine.
const allowLoopback = true
