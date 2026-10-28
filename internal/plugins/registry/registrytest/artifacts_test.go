package registrytest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/plugins"
)

func TestBinName(t *testing.T) {
	tests := []struct {
		platform plugins.Platform
		want     string
	}{
		{plugins.PlatformWindowsX64, "checkmate.exe"},
		{plugins.PlatformLinuxX64, "checkmate"},
		{plugins.PlatformDarwinArm64, "checkmate"},
	}

	for _, test := range tests {
		t.Run(string(test.platform), func(t *testing.T) {
			assert.Equal(t, test.want, BinName(test.platform, "checkmate"))
		})
	}
}
