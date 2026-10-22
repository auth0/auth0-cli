package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// downloadHosts are the hosts that plugins are downloaded from: GitHub releases,
// which redirect from github.com to the other two.
var downloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
}

const (
	// The number of redirects that a download follows at most.
	maxRedirects = 10

	// The timeouts that apply before the body of a download arrives. The body
	// itself has no timeout of its own, because artifacts are big and the caller
	// can cancel the context.
	connectTimeout = 30 * time.Second
	headerTimeout  = 60 * time.Second
)

// checkDownloadURL reports an error that wraps ErrDownloadHost unless raw is an
// https URL, without credentials and on the standard port, on a download host.
// A build made with the plugintest tag also accepts the loopback addresses
// 127.0.0.1, localhost and [::1], over http or https and on any port.
func checkDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w (%q is not a valid URL)", ErrDownloadHost, raw)
	}

	host := strings.ToLower(parsed.Hostname())

	if allowLoopback && (parsed.Scheme == "http" || parsed.Scheme == "https") && slices.Contains([]string{"127.0.0.1", "localhost", "::1"}, host) {
		return nil
	}

	switch {
	case parsed.Scheme != "https":
		return fmt.Errorf("%w (%q does not use https)", ErrDownloadHost, raw)
	case parsed.User != nil:
		return fmt.Errorf("%w (%q has a user name or password)", ErrDownloadHost, raw)
	case parsed.Port() != "" && parsed.Port() != "443":
		return fmt.Errorf("%w (%q uses port %s)", ErrDownloadHost, raw, parsed.Port())
	case !slices.Contains(downloadHosts, host):
		return fmt.Errorf("%w (%q is on %s)", ErrDownloadHost, raw, host)
	}

	return nil
}

// httpClient returns the client to download with. It always checks every
// redirect against the download hosts, whatever client was configured.
func (s *Store) httpClient() *http.Client {
	var client http.Client

	if s.HTTP != nil {
		client = *s.HTTP
	} else {
		client = http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   connectTimeout,
			ResponseHeaderTimeout: headerTimeout,
		}}
	}

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("%w (stopped after %d redirects)", ErrDownloadFailed, maxRedirects)
		}

		return checkDownloadURL(req.URL.String())
	}

	return &client
}

// download fetches the artifact into a new file at dest and checks it. The URL
// and every redirect must be allowed, the status must be 200, and the body is
// read while it is counted: no more than the size in the record is ever read.
// The file must then have exactly that size and the SHA-256 of the record. When
// any check fails, the error is returned and dest is removed.
//
// It returns the file that was written and hashed, open and positioned at its
// start, so that what is unpacked is the content that was checked and not
// whatever the path holds later. The caller closes it.
func (s *Store) download(ctx context.Context, artifact registry.Artifact, dest string) (_ *os.File, err error) {
	if err := checkDownloadURL(artifact.URL); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare the download of %s: %w", artifact.URL, err)
	}

	// The archive is hashed as it is served, so the server must not recompress it.
	request.Header.Set("Accept-Encoding", "identity")

	client := s.httpClient()

	// A client that was made for this download takes its connections with it.
	if s.HTTP == nil {
		defer client.CloseIdleConnections()
	}

	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, ErrDownloadHost) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, fmt.Errorf("failed to download %s: %w", artifact.URL, err)
		}

		return nil, fmt.Errorf("%w (%s: %w)", ErrDownloadFailed, artifact.URL, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w (%s answered %s)", ErrDownloadFailed, artifact.URL, response.Status)
	}

	if response.ContentLength > artifact.Size {
		return nil, fmt.Errorf("%w (%s announces %d bytes and the registry lists %d)", ErrDownloadTooLarge, artifact.URL, response.ContentLength, artifact.Size)
	}

	file, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", dest, err)
	}

	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(dest)
		}
	}()

	hasher := sha256.New()

	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(response.Body, artifact.Size+1))
	if err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrDownloadFailed, artifact.URL, err)
	}

	switch {
	case written > artifact.Size:
		return nil, fmt.Errorf("%w (%s has more than the %d bytes the registry lists)", ErrDownloadTooLarge, artifact.URL, artifact.Size)
	case written < artifact.Size:
		return nil, fmt.Errorf("%w (%s has %d bytes and the registry lists %d)", ErrDownloadSizeMismatch, artifact.URL, written, artifact.Size)
	case hex.EncodeToString(hasher.Sum(nil)) != artifact.SHA256:
		return nil, fmt.Errorf("%w (%s)", ErrDownloadHashMismatch, artifact.URL)
	}

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to read %s back: %w", dest, err)
	}

	return file, nil
}
