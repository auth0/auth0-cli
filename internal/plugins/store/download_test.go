package store_test

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// countingTransport counts the bytes of response bodies that are read, and hides
// the announced length of the body.
type countingTransport struct {
	inner http.RoundTripper
	read  *atomic.Int64
}

func (c countingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.inner.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	response.ContentLength = -1
	response.Body = countingBody{ReadCloser: response.Body, read: c.read}

	return response, nil
}

type countingBody struct {
	io.ReadCloser
	read *atomic.Int64
}

func (c countingBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.read.Add(int64(n))

	return n, err
}

func TestDownload_Failures(t *testing.T) {
	tests := []struct {
		name string
		want error

		// Serve changes what the server does for the archive of 1.0.0.
		serve func(f *fixture, record registry.VersionRecord)

		// Index changes the index before it is used.
		index func(index *registry.Index)
	}{
		{"a hash that differs", store.ErrDownloadHashMismatch, func(f *fixture, record registry.VersionRecord) {
			f.server.Set(artifactPath("1.0.0"), repeated('x', int(record.Artifacts[0].Size)))
		}, nil},
		{"fewer bytes than the registry lists", store.ErrDownloadSizeMismatch, func(f *fixture, record registry.VersionRecord) {
			f.server.Set(artifactPath("1.0.0"), repeated('x', int(record.Artifacts[0].Size)-1))
		}, nil},
		{"more bytes than the registry lists", store.ErrDownloadTooLarge, func(f *fixture, record registry.VersionRecord) {
			f.server.Set(artifactPath("1.0.0"), repeated('x', int(record.Artifacts[0].Size)+1))
		}, nil},
		{"a connection that is cut", store.ErrDownloadFailed, func(f *fixture, _ registry.VersionRecord) {
			f.server.Truncate(artifactPath("1.0.0"))
		}, nil},
		{"a status other than 200", store.ErrDownloadFailed, func(f *fixture, _ registry.VersionRecord) {
			f.server.Fail(artifactPath("1.0.0"), http.StatusInternalServerError)
		}, nil},
		{"a file that is gone", store.ErrDownloadFailed, func(f *fixture, _ registry.VersionRecord) {
			f.server.Set(artifactPath("1.0.0"), nil)
			f.server.Fail(artifactPath("1.0.0"), http.StatusNotFound)
		}, nil},
		{"a redirect to http", store.ErrDownloadHost, func(f *fixture, _ registry.VersionRecord) {
			f.server.Redirect(artifactPath("1.0.0"), "http://github.com/elsewhere")
		}, nil},
		{"a redirect to a host that is not allowed", store.ErrDownloadHost, func(f *fixture, _ registry.VersionRecord) {
			f.server.Redirect(artifactPath("1.0.0"), "https://evil.example.com/plugin.tar.gz")
		}, nil},
		{"a redirect to a look-alike host", store.ErrDownloadHost, func(f *fixture, _ registry.VersionRecord) {
			f.server.Redirect(artifactPath("1.0.0"), "https://objects.githubusercontent.com.evil.example.com/plugin.tar.gz")
		}, nil},
		{"a redirect through an allowed host to one that is not", store.ErrDownloadHost, func(f *fixture, _ registry.VersionRecord) {
			f.server.Redirect(artifactPath("1.0.0"), "https://objects.githubusercontent.com/hop")
			f.server.Redirect("/hop", "https://evil.example.com/plugin.tar.gz")
		}, nil},
		{"a redirect that never ends", store.ErrDownloadFailed, func(f *fixture, _ registry.VersionRecord) {
			f.server.Redirect(artifactPath("1.0.0"), "https://github.com"+artifactPath("1.0.0"))
		}, nil},
		{"a URL on another host", store.ErrDownloadHost, nil, func(index *registry.Index) {
			index.Plugins[0].Versions[0].Artifacts[0].URL = "https://example.com/checkmate.tar.gz"
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			record := f.release("1.0.0")
			index := f.index(record)

			if test.serve != nil {
				test.serve(f, record)
			}

			if test.index != nil {
				test.index(index)
			}

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			require.ErrorIs(t, err, test.want)
			assert.NoFileExists(t, f.lockfilePath())
			assert.Empty(t, f.installedVersions())
			assert.Empty(t, f.leftovers(), "the temporary file was deleted")
		})
	}
}

func TestDownload_FollowsRedirectsBetweenAllowedHosts(t *testing.T) {
	f := newFixture(t)
	record := f.release("1.0.0")
	index := f.index(record)

	// The archive moves to the last hop, as it does on GitHub.
	archive := f.archiveOf(t, record)
	f.server.Set("/release-asset/2", archive)
	f.server.Redirect(artifactPath("1.0.0"), "https://objects.githubusercontent.com/release-asset/1")
	f.server.Redirect("/release-asset/1", "https://release-assets.githubusercontent.com/release-asset/2")

	f.install(index, "")

	assert.Equal(t, 1, f.server.Requests("/release-asset/2"))
}

func TestDownload_StopsReadingAtTheListedSize(t *testing.T) {
	f := newFixture(t)
	record := f.release("1.0.0")
	index := f.index(record)

	size := record.Artifacts[0].Size
	f.server.Set(artifactPath("1.0.0"), repeated('x', int(size)*50))

	var read atomic.Int64

	client := f.server.AnyHostClient()
	client.Transport = countingTransport{inner: client.Transport, read: &read}
	f.store.HTTP = client

	_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

	require.ErrorIs(t, err, store.ErrDownloadTooLarge)
	assert.LessOrEqual(t, read.Load(), size+1, "the body is cut off while it streams, not read in full and then measured")
}
