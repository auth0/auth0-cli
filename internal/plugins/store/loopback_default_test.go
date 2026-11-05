//go:build !plugintest

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoopback_IsRejectedWithoutThePlugintestTag(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1:9/plugin.tar.gz", "https://127.0.0.1:9/plugin.tar.gz", "http://localhost/plugin.tar.gz", "http://[::1]:9/plugin.tar.gz"} {
		assert.ErrorIs(t, checkDownloadURL(url), ErrDownloadHost, url)
	}
}
