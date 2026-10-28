package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// The limits on what the client reads, so that a misbehaving registry cannot
// make the CLI use unbounded memory or disk.
const (
	maxIndexBytes     = 8 << 20
	maxSignatureBytes = 16 << 10
	maxStateBytes     = 16 << 10
)

const (
	// The timeout of a single request of the default HTTP client.
	requestTimeout = 30 * time.Second

	// The number of redirects that a request follows at most.
	maxRedirects = 10
)

// defaultHTTPClient is used when a Client has no HTTP client. It honors the
// proxy settings of the environment, such as HTTPS_PROXY.
var defaultHTTPClient = &http.Client{
	Timeout:   requestTimeout,
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, TLSHandshakeTimeout: requestTimeout},
}

// Client fetches the index of a registry, verifies it and keeps a copy on disk.
//
// The index is only ever used after its signature, schema, expiry and version
// were checked. Apart from the lock file, nothing is written to Dir before all
// of those checks passed, and a failed refresh leaves the cached index as it was.
type Client struct {
	Registry Registry

	// Keys are the keys that the signature of the index must verify against.
	Keys KeySet

	// HTTP is the client to fetch with. When it is nil, a client with a 30
	// second timeout that honors HTTPS_PROXY is used. Whatever the client is, it
	// follows redirects only to https URLs.
	HTTP *http.Client

	// Dir is where the cached index is kept, which is
	// <plugins dir>/registries/<name>.
	Dir string

	// Now returns the current time. When it is nil, time.Now is used.
	Now func() time.Time
}

// Status describes the cached index of a registry.
type Status struct {
	// LastRefresh is when the index was last downloaded and verified.
	LastRefresh time.Time

	// Stale is true when the index is older than the check interval of its
	// policy. Callers that install or update a plugin need an index that is not
	// stale.
	Stale bool

	// Warning explains in a sentence why the index is stale, and is empty when
	// there is nothing to warn about.
	Warning string

	// Runnable is true when plugins that are already installed may keep running,
	// which is when the index has not expired and was refreshed within the
	// offline grace period of its policy.
	Runnable bool
}

// NewClient returns a client for the registry that keeps its cache in dir and
// trusts the keys embedded in the CLI.
func NewClient(registry Registry, dir string) *Client {
	return &Client{Registry: registry, Keys: TrustedKeys(), Dir: dir}
}

// now returns the current time.
func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}

	return time.Now().UTC()
}

// Refresh downloads the index and its signature, verifies them, and stores
// them. It always goes to the network.
//
// It returns the index only when the signature verifies against Keys, the
// index follows the format, has not expired, and is not older than an index
// that was accepted before. Otherwise it returns one of ErrNoTrustedKeys,
// ErrSignatureInvalid, ErrUnsupportedFormat, ErrInvalidIndex, ErrExpired,
// ErrRollback, ErrOffline or ErrBusy, and the cache is left as it was.
//
// Refreshes are serialized across processes by a lock on a file in Dir, which is
// held while the state is read, the index is downloaded and verified, and the
// result is stored. Of two processes that refresh at the same time, the second
// waits for the first, so a process never replaces a newer index with an older
// one, and the index, signature and state in the cache always belong together.
// Load does not take the lock unless the cache fails verification, see Load.
func (c *Client) Refresh(ctx context.Context) (*Index, error) {
	index, _, err := c.refresh(ctx)

	return index, err
}

// refresh is Refresh that also returns the state that was stored.
func (c *Client) refresh(ctx context.Context) (*Index, state, error) {
	if len(c.Keys) == 0 {
		return nil, state{}, ErrNoTrustedKeys
	}

	release, err := c.lock(ctx)
	if err != nil {
		return nil, state{}, err
	}
	defer release()

	// The lock is what keeps another process from storing an index between the
	// moment the state is read here and the moment the new index is stored, so a
	// single read of the state is enough. What was accepted before is what the
	// downloaded index must not be older than.
	before, err := c.readState()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, state{}, err
	}

	var now time.Time

	indexData, signatureData, index, err := c.fetchVerified(ctx, func(index *Index) error {
		now = c.now()

		if !index.Expires.After(now) {
			return expiredError(index.Expires)
		}

		if index.Version < before.HighestVersion {
			return rollbackError(index.Version, before.HighestVersion)
		}

		return nil
	})
	if err != nil {
		return nil, state{}, err
	}

	current := state{HighestVersion: index.Version, LastRefresh: now}

	if err := c.persist(indexData, signatureData, current); err != nil {
		return nil, state{}, err
	}

	return index, current, nil
}

// lock takes the lock that serializes refreshes of this cache, creating the
// cache directory and the lock file when they do not exist, and returns the
// function that frees it. The lock is an exclusive lock on the lock file, which
// is created with restricted permissions. The operating system also frees it when
// the process ends, so a crash cannot leave it behind. Locks on separate handles
// of the same file exclude each other, also within one process.
//
// It tries without blocking and waits lockPollInterval between attempts, until
// the context is done or lockTimeout has passed. It then returns an error that
// wraps ErrBusy, and also wraps the error of the context when that is what ended
// the wait.
func (c *Client) lock(ctx context.Context) (release func(), err error) {
	if err := os.MkdirAll(c.Dir, cacheDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create the registry directory: %w", err)
	}

	if err := os.Chmod(c.Dir, cacheDirPerm); err != nil {
		return nil, fmt.Errorf("failed to restrict the registry directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(c.Dir, lockFileName), os.O_RDWR|os.O_CREATE, cacheFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open the registry lock: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	ticker := time.NewTicker(lockPollInterval)
	defer ticker.Stop()

	for {
		locked, err := tryLock(file)
		if err != nil {
			_ = file.Close()

			return nil, fmt.Errorf("failed to lock the registry cache: %w", err)
		}

		if locked {
			// Freeing the lock does what it can and reports nothing, as closing the
			// file frees it too.
			return func() {
				unlock(file)
				_ = file.Close()
			}, nil
		}

		select {
		case <-waitCtx.Done():
			_ = file.Close()

			if ctx.Err() != nil {
				return nil, fmt.Errorf("%w (%w)", ErrBusy, ctx.Err())
			}

			return nil, ErrBusy
		case <-ticker.C:
		}
	}
}

// fetchVerified downloads the index and its signature, verifies them, parses
// the index and runs accept on it. If the signature does not verify, it
// downloads both once more, because the registry may have published a new index
// between the two requests.
//
// When the first attempt failed the signature check, the second attempt must not
// hide that: whatever else goes wrong with it, such as an unreachable registry
// or an older index, the error that is returned still wraps the signature
// failure, so that the caller does not take the registry for unreachable and
// fall back to the cache.
func (c *Client) fetchVerified(ctx context.Context, accept func(index *Index) error) (indexData, signatureData []byte, index *Index, err error) {
	signatureURL, err := signatureURL(c.Registry.URL)
	if err != nil {
		return nil, nil, nil, err
	}

	indexData, signatureData, index, err = c.fetchAttempt(ctx, signatureURL, accept)
	if err == nil || !errors.Is(err, ErrSignatureInvalid) {
		return indexData, signatureData, index, err
	}

	first := err

	indexData, signatureData, index, err = c.fetchAttempt(ctx, signatureURL, accept)
	switch {
	case err == nil:
		return indexData, signatureData, index, nil
	case errors.Is(err, ErrSignatureInvalid):
		return nil, nil, nil, err
	default:
		return nil, nil, nil, fmt.Errorf("%w (retry also failed: %v)", first, err)
	}
}

// fetchAttempt downloads the index and its signature once, and returns them
// with the parsed index when the signature verifies, the index is valid and
// accept approves it.
func (c *Client) fetchAttempt(ctx context.Context, signatureURL string, accept func(index *Index) error) (indexData, signatureData []byte, index *Index, err error) {
	indexData, signatureData, err = c.fetchPair(ctx, signatureURL)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := c.Keys.Verify(indexData, signatureData); err != nil {
		return nil, nil, nil, err
	}

	index, err = ParseIndex(indexData)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := accept(index); err != nil {
		return nil, nil, nil, err
	}

	return indexData, signatureData, index, nil
}

// fetchPair downloads the index, and then its signature.
func (c *Client) fetchPair(ctx context.Context, signatureURL string) (indexData, signatureData []byte, err error) {
	indexData, err = c.get(ctx, c.Registry.URL, maxIndexBytes)
	if err != nil {
		return nil, nil, err
	}

	signatureData, err = c.get(ctx, signatureURL, maxSignatureBytes)
	if err != nil {
		return nil, nil, err
	}

	return indexData, signatureData, nil
}

// Load reads the cached index from disk and verifies it again, because the
// cache is not trusted: it returns the index only when its signature still
// verifies, it has not expired and it is not older than an index that was
// accepted before. It never goes to the network.
//
// It returns ErrNotCached when there is no complete cache. When the index has
// expired it returns ErrExpired and no index, but the status is still filled in
// so that it can be shown.
//
// Load does not take the lock while the cache verifies. When it does not, with
// ErrSignatureInvalid or ErrRollback, Load may have read the files while another
// process was replacing them, so it waits for that process to finish, as long
// as the lock timeout or ctx allow, and reads the cache once more. The result of
// the second read is the one that is returned.
//
// The lock is released before Load returns, so a caller must not hold it.
// Nothing that holds the lock calls Load: refresh reads the state with
// readState and never reads the cache with loadCached.
func (c *Client) Load(ctx context.Context) (*Index, Status, error) {
	index, current, err := c.loadCached()
	if errors.Is(err, ErrSignatureInvalid) || errors.Is(err, ErrRollback) {
		// Read the cache again while holding the lock, so that no refresh is
		// replacing its files. When the lock cannot be taken, the result of the
		// first read stands, because a cache that failed verification is not
		// trusted whether or not the failure could be explained.
		if release, lockErr := c.lock(ctx); lockErr == nil {
			index, current, err = c.loadCached()

			release()
		}
	}

	if index == nil {
		return nil, Status{}, err
	}

	status := c.status(index, current)
	if err != nil {
		return nil, status, err
	}

	return index, status, nil
}

// loadCached reads the cached index and verifies it again, and returns it with
// the state it was stored with. When the index is genuine but must not be used,
// because it expired or is older than the highest version seen, it is returned
// together with the error so that Load can still report its status. Callers must
// not use an index that comes with an error.
func (c *Client) loadCached() (*Index, state, error) {
	if len(c.Keys) == 0 {
		return nil, state{}, ErrNoTrustedKeys
	}

	indexData, err := c.readCacheFile(indexFileName, maxIndexBytes)
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	signatureData, err := c.readCacheFile(signatureFileName, maxSignatureBytes)
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	current, err := c.readState()
	if err != nil {
		return nil, state{}, notCachedOr(err)
	}

	if err := c.Keys.Verify(indexData, signatureData); err != nil {
		return nil, state{}, err
	}

	index, err := ParseIndex(indexData)
	if err != nil {
		return nil, state{}, err
	}

	if !index.Expires.After(c.now()) {
		return index, current, expiredError(index.Expires)
	}

	if index.Version < current.HighestVersion {
		return index, current, rollbackError(index.Version, current.HighestVersion)
	}

	return index, current, nil
}

// EnsureFresh returns the cached index when it is recent enough, and refreshes
// it first when it is missing, expired, or older than the check interval of its
// policy.
//
// When the registry cannot be reached, which includes the deadline of ctx
// passing, but the cached index is still usable, it returns the cached index with
// Stale set and the reason in Warning, and no error. It does the same when the
// registry serves an older index than one that was accepted before, and when
// another process holds the lock because it is already refreshing, in which case
// the cache is the index that process is about to replace. In none of these cases
// is the time of the last refresh advanced, so a registry that keeps failing, or
// keeps serving old data, makes the index unrunnable once the offline grace
// period has passed. The cached index is judged again after the refresh failed,
// so one that expired while the refresh was running is not returned: the error is
// ErrExpired and there is no index.
//
// Any other failure to refresh, a cancelled ctx among them, is returned as an
// error and never falls back to the cache, so a failed signature check, for
// example, is not hidden. Callers that install or update a plugin must require
// that Stale is false.
func (c *Client) EnsureFresh(ctx context.Context) (*Index, Status, error) {
	index, status, loadErr := c.Load(ctx)
	if loadErr == nil && !status.Stale {
		return index, status, nil
	}

	refreshed, current, err := c.refresh(ctx)
	if err == nil {
		return refreshed, c.status(refreshed, current), nil
	}

	switch {
	case !canUseCacheAfter(err):
		return nil, status, err
	case errors.Is(loadErr, ErrNotCached):
		return nil, status, err
	case loadErr != nil:
		return nil, status, fmt.Errorf("%w (the cached index cannot be used: %w)", err, loadErr)
	}

	// The refresh may have taken long enough for the index to expire, so what
	// the cache is worth is worked out again.
	status = c.status(index, state{LastRefresh: status.LastRefresh})
	if !index.Expires.After(c.now()) {
		return nil, status, fmt.Errorf("%w (the registry could not be refreshed: %v)", expiredError(index.Expires), err)
	}

	reason := err.Error()
	if errors.Is(err, ErrBusy) {
		reason = "Another auth0 process is refreshing the registry"
	}

	warning := fmt.Sprintf("%s (using the cached index from %s ago)", reason, describeAge(c.now().Sub(status.LastRefresh)))
	if !status.Runnable {
		warning = status.Warning + " " + warning
	}

	status.Warning = warning

	return index, status, nil
}

// canUseCacheAfter reports whether a verified cached index may stand in for the
// index that a failed refresh did not deliver, which is the case when the
// registry cannot be reached, serves an older index, or is being refreshed by
// another process. A cancelled context is the caller giving up, so it does not
// count.
func canUseCacheAfter(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	return errors.Is(err, ErrOffline) || errors.Is(err, ErrRollback) || errors.Is(err, ErrBusy)
}

// status works out the status of an index that was last refreshed as current
// says.
func (c *Client) status(index *Index, current state) Status {
	policy := index.Policy.WithDefaults()
	now := c.now()
	age := now.Sub(current.LastRefresh)
	grace := time.Duration(policy.OfflineGraceDays) * 24 * time.Hour
	interval := time.Duration(policy.CheckIntervalHours) * time.Hour

	status := Status{
		LastRefresh: current.LastRefresh,
		Stale:       age > interval,
		Runnable:    index.Expires.After(now) && age <= grace,
	}

	switch {
	case status.Runnable && status.Stale:
		status.Warning = fmt.Sprintf("The registry index was last refreshed %s ago.", describeAge(age))
	case !status.Runnable:
		status.Warning = fmt.Sprintf("The registry index was last refreshed %s ago, so installed plugins cannot run until it is refreshed.", describeAge(age))
	}

	return status
}

// Versions returns the versions of a plugin from the index. Versions that are
// listed in a separate file are downloaded from the path that the plugin's ref
// has, relative to the URL of the index. The file is checked against the size
// limit, the hash pinned in the index and the same rules as the index itself.
//
// The plugin must come from an index that Refresh, Load or EnsureFresh returned.
func (c *Client) Versions(ctx context.Context, plugin Plugin) ([]VersionRecord, error) {
	if plugin.Ref == nil {
		return slices.Clone(plugin.Versions), nil
	}

	base, err := url.Parse(c.Registry.URL)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid URL: %w", c.Registry.URL, err)
	}

	// The query of the registry URL goes with the file, like it goes with the
	// signature, because a mirror may need it to serve any file.
	target := base.ResolveReference(&url.URL{Path: plugin.Ref.Path})
	target.RawQuery = base.RawQuery

	data, err := c.get(ctx, target.String(), maxIndexBytes)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != plugin.Ref.SHA256 {
		return nil, fmt.Errorf("%w (%s has the hash %s and the index pins %s)", ErrHashMismatch, plugin.Ref.Path, got, plugin.Ref.SHA256)
	}

	return parseVersionsFile(data, plugin.Latest, plugin.Ref.Path)
}

// httpClient returns the client to fetch with. It always checks redirects,
// whatever client was configured.
func (c *Client) httpClient() *http.Client {
	client := *defaultHTTPClient
	if c.HTTP != nil {
		client = *c.HTTP
	}

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}

		return checkURL(req.URL.String())
	}

	return &client
}

// get downloads rawURL, which must be an https URL, and returns its body when
// the server answers 200 and the body is at most limit bytes.
func (c *Client) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if err := checkURL(rawURL); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare the request for %s: %w", rawURL, err)
	}

	response, err := c.httpClient().Do(request)
	if err != nil {
		// A timeout of the caller counts as an unreachable registry, like the
		// timeout of the HTTP client. A cancellation is the caller giving up, so it
		// is not reported as an unreachable registry.
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, ErrInsecureURL) {
			return nil, fmt.Errorf("failed to fetch %s: %w", rawURL, err)
		}

		return nil, fmt.Errorf("%w at %s: %w", ErrOffline, rawURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w at %s: the server answered %s", ErrOffline, rawURL, response.Status)
	}

	data, err := readCapped(response.Body, limit)
	switch {
	case errors.Is(err, errTooLarge):
		return nil, newInvalidError(rawURL, []string{fmt.Sprintf("(root): larger than the allowed %d bytes", limit)})
	case err != nil:
		return nil, fmt.Errorf("%w at %s: %w", ErrOffline, rawURL, err)
	}

	return data, nil
}

// signatureURL returns the URL of the signature that belongs to the index at
// indexURL, which is the same URL with .sig added to the path and the query kept.
func signatureURL(indexURL string) (string, error) {
	parsed, err := url.Parse(indexURL)
	if err != nil {
		return "", fmt.Errorf("%q is not a valid URL: %w", indexURL, err)
	}

	parsed.Path += ".sig"
	parsed.RawPath = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}

// notCachedOr turns the error of a missing cache file into ErrNotCached, and
// returns any other error as it is.
func notCachedOr(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotCached
	}

	return err
}

// expiredError returns an error that wraps ErrExpired.
func expiredError(expires time.Time) error {
	return fmt.Errorf("%w on %s", ErrExpired, expires.Format(time.RFC3339))
}

// rollbackError returns an error that wraps ErrRollback.
func rollbackError(version, highest int) error {
	return fmt.Errorf("%w (version %d is older than version %d)", ErrRollback, version, highest)
}

// describeAge describes a duration in whole days or hours.
func describeAge(age time.Duration) string {
	const day = 24 * time.Hour

	switch {
	case age >= 2*day:
		return fmt.Sprintf("%d days", age/day)
	case age >= 2*time.Hour:
		return fmt.Sprintf("%d hours", age/time.Hour)
	case age >= time.Hour:
		return "1 hour"
	default:
		return "less than an hour"
	}
}
