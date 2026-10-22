package registry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

// fetchMetadata runs the TUF client workflow against the registry, starting from
// the root that verifiedRoot found, and returns what it accepted together with
// the index. It does not write to the cache and does not check the index
// itself.
//
// Go-tuf is given a temporary directory as its cache, seeded with a copy of the
// metadata that is cached, so that it applies its rollback checks against it. It
// can write there, and the directory goes away afterwards, so go-tuf never
// touches the cache of the registry. What it accepted is read back from that
// directory, and the roots it walked from the fetcher.
func (c *Client) fetchMetadata(ctx context.Context, chain rootChain, now time.Time) (verified, error) {
	base, err := c.baseURL()
	if err != nil {
		return verified{}, err
	}

	scratch, err := os.MkdirTemp("", "auth0-registry-")
	if err != nil {
		return verified{}, fmt.Errorf("failed to create a temporary directory: %w", err)
	}
	defer os.RemoveAll(scratch)

	scratchMetadata := filepath.Join(scratch, metadataDirName)
	if err := c.seedMetadata(scratchMetadata); err != nil {
		return verified{}, err
	}

	remote := *base
	remote.RawQuery, remote.Fragment = "", ""

	cfg, err := config.New(remote.String(), chain.Bytes)
	if err != nil {
		return verified{}, fmt.Errorf("failed to prepare the registry client: %w", err)
	}

	fetch := &fetcher{ctx: ctx, client: c, query: base.RawQuery, targetsPrefix: cfg.RemoteTargetsURL + "/"}

	cfg.Fetcher = fetch
	cfg.LocalMetadataDir = scratchMetadata
	cfg.LocalTargetsDir = filepath.Join(scratch, "targets")
	cfg.RootMaxLength = maxMetadataBytes
	cfg.TimestampMaxLength = maxMetadataBytes
	cfg.SnapshotMaxLength = maxMetadataBytes
	cfg.TargetsMaxLength = maxMetadataBytes

	client, err := updater.New(cfg)
	if err != nil {
		return verified{}, fmt.Errorf("failed to start the registry client: %w", err)
	}

	client.UnsafeSetRefTime(now)

	if err := client.Refresh(); err != nil {
		return verified{}, tufError(client, now, err)
	}

	target, found := client.GetTopLevelTargets()[targetName]
	if !found {
		return verified{}, invalid("the signed %s does not list %s", targetsFileName, targetName)
	}

	if target.Length > maxIndexBytes {
		return verified{}, newInvalidError(targetName, []string{fmt.Sprintf("(root): larger than the allowed %d bytes", maxIndexBytes)})
	}

	accepted := verified{Roots: make(map[int64][]byte)}

	// The index is only downloaded when the cached one is not the one that the
	// signed metadata names.
	accepted.Index, err = c.readCacheFile(indexFileName, maxIndexBytes)
	if err != nil || target.VerifyLengthHashes(accepted.Index) != nil {
		_, accepted.Index, err = client.DownloadTarget(target, "", "")
		if err != nil {
			return verified{}, tufError(client, now, err)
		}
	}

	for file, data := range map[string]*[]byte{
		rootFileName:      &accepted.Root,
		timestampFileName: &accepted.Timestamp,
		snapshotFileName:  &accepted.Snapshot,
		targetsFileName:   &accepted.Targets,
	} {
		*data, err = os.ReadFile(filepath.Join(scratchMetadata, file))
		if err != nil {
			return verified{}, fmt.Errorf("failed to read the metadata that the registry client accepted: %w", err)
		}
	}

	final, err := metadata.Root().FromBytes(accepted.Root)
	if err != nil {
		return verified{}, fmt.Errorf("failed to read the root that the registry client accepted: %w", err)
	}

	accepted.FinalRoot = final.Signed.Version

	for _, data := range fetch.roots {
		root, err := metadata.Root().FromBytes(data)
		if err != nil {
			return verified{}, fmt.Errorf("failed to read a root that the registry client accepted: %w", err)
		}

		accepted.Roots[root.Signed.Version] = data
	}

	return accepted, nil
}

// seedMetadata copies the cached timestamp, snapshot and targets metadata to
// dir, so that go-tuf applies its checks against the versions the client has
// seen. The root is not copied, because go-tuf is handed the root chain. A file
// that is not in the cache is left out, and one that is damaged is an error,
// because go-tuf would stop on it, and leaving it out would let a damaged cache
// reset the protection against older metadata.
func (c *Client) seedMetadata(dir string) error {
	if err := os.MkdirAll(dir, cacheDirPerm); err != nil {
		return fmt.Errorf("failed to create a temporary directory: %w", err)
	}

	for _, file := range []string{timestampFileName, snapshotFileName, targetsFileName} {
		name := metadataFilePath(file)

		data, err := c.readCacheFile(name, maxMetadataBytes)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return err
		}

		var parseErr error

		switch file {
		case timestampFileName:
			_, parseErr = metadata.Timestamp().FromBytes(data)
		case snapshotFileName:
			_, parseErr = metadata.Snapshot().FromBytes(data)
		default:
			_, parseErr = metadata.Targets().FromBytes(data)
		}

		if parseErr != nil {
			return fmt.Errorf("the registry metadata %s is damaged, remove %s to start over: %w", c.cachePath(name), c.Dir, parseErr)
		}

		if err := os.WriteFile(filepath.Join(dir, file), data, cacheFilePerm); err != nil {
			return fmt.Errorf("failed to seed the registry client: %w", err)
		}
	}

	return nil
}

// timestampExpiredError is the error for a timestamp that has expired. It is
// the one expiry that a verified cached index may stand in for, because a
// registry that is not re-signed looks the same as one that cannot be reached.
// It wraps ErrExpired.
type timestampExpiredError struct {
	expires time.Time
}

// Error returns the message of the error.
func (e *timestampExpiredError) Error() string {
	return expiredMetadataError(timestampFileName, e.expires).Error()
}

// Unwrap returns ErrExpired.
func (e *timestampExpiredError) Unwrap() error {
	return ErrExpired
}

// tufError turns an error of go-tuf into one of the errors of this package. The
// text of go-tuf is kept in the message, but the kind of the error is decided by
// its type.
//
//   - A failure to reach the registry, which the fetcher already reports as
//     ErrOffline, a URL that is not https, and a cancelled context are returned
//     as they are.
//   - Metadata that is expired is ErrExpired, and the expiry of the timestamp
//     alone is also a timestampExpiredError.
//   - Metadata whose version is not the one that was expected, or is older than
//     one seen before, is ErrRollback. That includes files that the registry
//     serves out of step.
//   - Everything else, which is insufficient signatures, a length or hash that
//     does not match, metadata that cannot be read, and anything that go-tuf
//     reports that is not known, is ErrSignatureInvalid, so that it never counts
//     as a registry that cannot be reached.
func tufError(client *updater.Updater, now time.Time, err error) error {
	var (
		expired *metadata.ErrExpiredMetadata
		version *metadata.ErrBadVersionNumber
	)

	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, ErrOffline), errors.Is(err, ErrInsecureURL):
		return err
	case errors.As(err, &expired):
		return expiryError(client.GetTrustedMetadataSet(), now, err)
	case errors.As(err, &version):
		return fmt.Errorf("%w (%v)", ErrRollback, err)
	default:
		return invalid("%v", err)
	}
}

// expiryError works out which piece of metadata expired. Go-tuf does not say,
// but it loads and checks the root, timestamp, snapshot and targets in that
// order, and keeps each one it loaded, expired or not, so the first one that is
// expired by now is the one that stopped it.
func expiryError(trusted trustedmetadata.TrustedMetadata, now time.Time, cause error) error {
	switch {
	case trusted.Root != nil && trusted.Root.Signed.IsExpired(now):
		return expiredMetadataError(rootFileName, trusted.Root.Signed.Expires)
	case trusted.Timestamp != nil && trusted.Timestamp.Signed.IsExpired(now):
		return &timestampExpiredError{expires: trusted.Timestamp.Signed.Expires}
	case trusted.Snapshot != nil && trusted.Snapshot.Signed.IsExpired(now):
		return expiredMetadataError(snapshotFileName, trusted.Snapshot.Signed.Expires)
	default:
		return fmt.Errorf("%w (%s expired: %v)", ErrExpired, targetsFileName, cause)
	}
}

// fetcher is the HTTP side of go-tuf. It downloads with the client of the
// registry, so the timeout, the https rule, the redirect checks, the proxy
// settings and the size limits are the ones that every other request has.
type fetcher struct {
	ctx    context.Context
	client *Client

	// Query is the query of the URL of the registry, which goes with every
	// request, because a mirror may need it to serve any file.
	query string

	// TargetsPrefix is the URL that the URLs of target files start with.
	targetsPrefix string

	// Roots are the roots that were downloaded, in the order they were served.
	roots [][]byte
}

// DownloadFile downloads urlPath, and fails with go-tuf's error for a length
// mismatch when the body is longer than maxLength or than the limit of this
// package for what it is, which is the index for a target file and metadata for
// the rest.
func (f *fetcher) DownloadFile(urlPath string, maxLength int64, _ time.Duration) ([]byte, error) {
	limit := min(maxLength, maxMetadataBytes)
	if strings.HasPrefix(urlPath, f.targetsPrefix) {
		limit = min(maxLength, maxIndexBytes)
	}

	target, err := url.Parse(urlPath)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid URL: %w", urlPath, err)
	}

	target.RawQuery = f.query

	data, err := f.client.download(f.ctx, target.String(), limit)
	if errors.Is(err, errTooLarge) {
		return nil, &metadata.ErrDownloadLengthMismatch{Msg: fmt.Sprintf("download failed for %s, the body is larger than the allowed %d bytes", urlPath, limit)}
	}

	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(path.Base(target.Path), ".root.json") {
		f.roots = append(f.roots, data)
	}

	return data, nil
}
