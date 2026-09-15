package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNpmSpec(t *testing.T) {
	p := Plugin{Install: Install{Type: InstallNPM, Package: "@auth0/auth0-checkmate", Version: "1.8.5"}}
	assert.Equal(t, "@auth0/auth0-checkmate@1.8.5", NpmSpec(p))
}

func TestSelectAsset(t *testing.T) {
	p := Plugin{Install: Install{
		Type: InstallGitHubRelease,
		Assets: []ReleaseAsset{
			{OS: "darwin", Arch: "arm64", URL: "u1", SHA256: "s1"},
			{OS: "linux", Arch: "amd64", URL: "u2", SHA256: "s2"},
		},
	}}

	asset, ok := SelectAsset(p, "linux", "amd64")
	require.True(t, ok)
	assert.Equal(t, "u2", asset.URL)

	_, ok = SelectAsset(p, "windows", "arm64")
	assert.False(t, ok)
}

func TestDownloadAndVerify(t *testing.T) {
	payload := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(payload)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "bin", "widget")
	err := downloadAndVerify(context.Background(), server.Client(), server.URL, hex.EncodeToString(sum[:]), dest)
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestDownloadAndVerifyRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered payload"))
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "widget")
	wrongSum := hex.EncodeToString(make([]byte, sha256.Size))
	err := downloadAndVerify(context.Background(), server.Client(), server.URL, wrongSum, dest)
	assert.ErrorIs(t, err, ErrChecksumMismatch)

	// A rejected download must not leave a binary behind.
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr))
}

func TestDownloadAndVerifyServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "widget")
	err := downloadAndVerify(context.Background(), server.Client(), server.URL, "abc", dest)
	assert.Error(t, err)
}
