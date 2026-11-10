package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// maxRootRotations is how many roots after the embedded one the client walks at
// most, which is the limit that go-tuf has too.
const maxRootRotations = 256

// targetName is the one target that the registry signs.
const targetName = "index.json"

// rootChain is the root that the client trusts, which is the last of a chain of
// roots that starts at the embedded root.
type rootChain struct {
	// Root is the last root of the chain, and Bytes its content.
	Root  *metadata.Metadata[metadata.RootType]
	Bytes []byte

	// Cached is how many of the roots after the embedded one were read from the
	// cache.
	Cached int
}

// verifiedRoot reads the embedded root and the roots in the cache, and returns
// the last root that is connected to the embedded one by signatures.
//
// The embedded root is the anchor of trust and must be signed by a threshold of
// its own root keys. Every cached root must have the version that follows the
// version before it, and be signed by a threshold of the root keys of the root
// before it and by a threshold of its own root keys. The walk stops at the first
// version that is not in the cache, and a cached root that does not verify is an
// error, not the end of the walk, so that a tampered cache is never used.
//
// The expiry of the roots is not checked, because a refresh may replace an
// expired root with a newer one that it downloads. Whoever uses the root checks
// that the last one has not expired.
//
// It returns ErrNoTrustedKeys when there is no usable embedded root, and an
// error that wraps ErrSignatureInvalid when a cached root does not verify. In
// that case the chain it returns is the part of the cached chain that did verify,
// which is the embedded root when none of it did, so that a refresh can walk the
// rest again over the network. Anything else that uses the chain must treat the
// error as the end of the matter.
func (c *Client) verifiedRoot() (rootChain, error) {
	data := c.trustedRoot()
	if len(data) == 0 {
		return rootChain{}, ErrNoTrustedKeys
	}

	anchor, err := metadata.Root().FromBytes(data)
	if err != nil {
		return rootChain{}, fmt.Errorf("%w (the embedded root is not valid: %v)", ErrNoTrustedKeys, err)
	}

	if err := anchor.VerifyDelegate(metadata.ROOT, anchor); err != nil {
		return rootChain{}, fmt.Errorf("%w (the embedded root is not signed by its own keys: %v)", ErrNoTrustedKeys, err)
	}

	chain := rootChain{Root: anchor, Bytes: data}

	for chain.Cached < maxRootRotations {
		version := chain.Root.Signed.Version + 1
		name := rootFilePath(version)

		data, err := c.readCacheFile(name, maxMetadataBytes)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}

		if err != nil {
			return rootChain{}, err
		}

		next, err := metadata.Root().FromBytes(data)
		if err != nil {
			return chain, c.invalidRoot(name, "is not valid: %v", err)
		}

		if next.Signed.Version != version {
			return chain, c.invalidRoot(name, "has version %d", next.Signed.Version)
		}

		if err := chain.Root.VerifyDelegate(metadata.ROOT, next); err != nil {
			return chain, c.invalidRoot(name, "is not signed by the root before it: %v", err)
		}

		if err := next.VerifyDelegate(metadata.ROOT, next); err != nil {
			return chain, c.invalidRoot(name, "is not signed by its own keys: %v", err)
		}

		chain.Root, chain.Bytes = next, data
		chain.Cached++
	}

	return chain, nil
}

// invalidRoot returns the error for a cached root that does not verify, which
// wraps ErrSignatureInvalid and names the directory to remove to start over.
func (c *Client) invalidRoot(name, format string, args ...any) error {
	return invalid("the cached root %s %s, remove %s to start over", c.cachePath(name), fmt.Sprintf(format, args...), c.Dir)
}

// loadCached reads the cached index and verifies it again, and returns it with
// the state it was stored with. Nothing is taken on trust: the root chain, the
// targets metadata and the index are checked against the keys of the embedded
// root and against each other, the way a refresh checks them, and only the
// expiry of the timestamp and of the snapshot is left out, because those are
// short lived and the client has no network to refresh them. Nor does it look
// at whether the timestamp and the snapshot belong to the targets metadata.
//
// When the index is genuine but must not be used, because something expired or
// the index is older than the highest version seen, it is returned together with
// the error so that Load can still report its status. Callers must not use an
// index that comes with an error.
func (c *Client) loadCached() (*Index, state, error) {
	if len(c.trustedRoot()) == 0 {
		return nil, state{}, ErrNoTrustedKeys
	}

	current, err := c.readState()
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	chain, err := c.verifiedRoot()
	if err != nil {
		return nil, state{}, err
	}

	targetsData, err := c.readCacheFile(metadataFilePath(targetsFileName), maxMetadataBytes)
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	signedTargets, err := metadata.Targets().FromBytes(targetsData)
	if err != nil {
		return nil, state{}, invalid("the cached %s is not valid: %v", targetsFileName, err)
	}

	if err := chain.Root.VerifyDelegate(metadata.TARGETS, signedTargets); err != nil {
		return nil, state{}, invalid("the cached %s is not signed by the targets keys of the root: %v", targetsFileName, err)
	}

	target, found := signedTargets.Signed.Targets[targetName]
	if !found {
		return nil, state{}, invalid("the cached %s does not list %s", targetsFileName, targetName)
	}

	indexData, err := c.readCacheFile(indexFileName, maxIndexBytes)
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	if err := target.VerifyLengthHashes(indexData); err != nil {
		return nil, state{}, invalid("the cached %s does not match %s: %v", indexFileName, targetsFileName, err)
	}

	index, err := ParseIndex(indexData)
	if err != nil {
		return nil, state{}, err
	}

	now := c.now()

	switch {
	case chain.Root.Signed.IsExpired(now):
		return index, current, expiredMetadataError(rootFileName, chain.Root.Signed.Expires)
	case signedTargets.Signed.IsExpired(now):
		return index, current, expiredMetadataError(targetsFileName, signedTargets.Signed.Expires)
	case !index.Expires.After(now):
		return index, current, expiredError(index.Expires)
	case index.Version < current.HighestVersion:
		return index, current, rollbackError(index.Version, current.HighestVersion)
	}

	return index, current, nil
}

// expiredMetadataError returns an error that wraps ErrExpired for a file of
// signed metadata.
func expiredMetadataError(file string, expires time.Time) error {
	return fmt.Errorf("%w (%s expired on %s)", ErrExpired, file, expires.Format(time.RFC3339))
}
