package registry

import (
	"crypto/ed25519"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// keyFiles holds the trusted public keys that ship with the CLI. The directory
// also holds a README, so that it exists in a build that embeds no key yet.
//
// TODO(DXCDT-2611): Before the first release that ships the plugin framework,
// add the production public key as keys/<keyId>.pub, where the key ID is the one
// that the signing job puts in index.json.sig. Also add the key that is used to
// rotate it, document the key ID in the release process, and check that
// TrustedKeys is no longer empty in a release build. Until then every
// verification fails with ErrNoTrustedKeys, which is the safe default.
//
//go:embed keys
var keyFiles embed.FS

// keyFileExtension is the extension of a file in keys that holds a public key.
const keyFileExtension = ".pub"

// TrustedKeys returns the public keys that this build of the CLI trusts to sign
// a registry index. The keys are embedded in the binary, and nothing at runtime
// can add to them or replace them.
//
// A build without any trusted key returns an empty set, and an empty set can
// verify nothing. It panics when an embedded key is malformed, which a test
// catches before a release, because a CLI that cannot read its own keys must not
// carry on.
func TrustedKeys() KeySet {
	keys, err := parseKeys(keyFiles, "keys")
	if err != nil {
		panic(err)
	}

	for id, key := range testKeys() {
		keys[id] = key
	}

	return keys
}

// parseKeys reads every .pub file in the directory dir of fsys. The key ID is
// the name of the file without the extension.
func parseKeys(fsys fs.FS, dir string) (KeySet, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read the embedded registry keys: %w", err)
	}

	keys := make(KeySet)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || path.Ext(name) != keyFileExtension {
			continue
		}

		content, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("failed to read the embedded registry key %s: %w", name, err)
		}

		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(content)))
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("the embedded registry key %s is not the base64 of a %d byte Ed25519 public key", name, ed25519.PublicKeySize)
		}

		keys[strings.TrimSuffix(name, keyFileExtension)] = key
	}

	return keys, nil
}
