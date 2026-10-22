//go:build plugintest

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoopback_IsAllowedWithThePlugintestTag(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1:9/plugin.tar.gz", "https://127.0.0.1:9/plugin.tar.gz", "http://localhost/plugin.tar.gz", "http://[::1]:9/plugin.tar.gz"} {
		assert.NoError(t, checkDownloadURL(url), url)
	}

	for _, url := range []string{"http://example.com/plugin.tar.gz", "http://127.0.0.1.example.com/plugin.tar.gz", "ftp://127.0.0.1/plugin.tar.gz"} {
		assert.ErrorIs(t, checkDownloadURL(url), ErrDownloadHost, url)
	}
}
