package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  Version
		ok    bool
	}{
		{"1.39.0", Version{1, 39, 0}, true},
		{"v1.39.0", Version{1, 39, 0}, true},
		{"1.39.0-beta.3", Version{1, 39, 0}, true},
		{"1.39.0+build.5", Version{1, 39, 0}, true},
		{"0.0.0", Version{}, true},
		{"", Version{}, false},
		{"dev", Version{}, false},
		{"1.39", Version{}, false},
		{"1.39.0.1", Version{}, false},
		{"01.39.0", Version{}, false},
		{"1.-3.0", Version{}, false},
		{"1.39.x", Version{}, false},
		{"99999999999999999999.0.0", Version{}, false},
		{"1..0", Version{}, false},
		{"1.0.", Version{}, false},
		{"1_0.0.0", Version{}, false},
		{"1.0.0_1", Version{}, false},
		{"0x1.0.0", Version{}, false},
		{"1.0.00", Version{}, false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, ok := ParseVersion(test.input)

			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestVersion_String(t *testing.T) {
	assert.Equal(t, "1.39.0", Version{1, 39, 0}.String())
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.10.0", "1.9.0", 1},
		{"2.0.0", "10.0.0", -1},
		{"1.2.3-beta.1", "1.2.3", 0},
	}

	for _, test := range tests {
		a, ok := ParseVersion(test.a)
		require.True(t, ok)

		b, ok := ParseVersion(test.b)
		require.True(t, ok)

		assert.Equal(t, test.want, a.Compare(b), "%s against %s", test.a, test.b)
	}
}

func TestIsPrereleaseAndFeatureEnabled(t *testing.T) {
	tests := []struct {
		version    string
		prerelease bool
		enabled    bool
	}{
		{"", false, true},
		{"dev", false, true},
		{"1.39.0", false, false},
		{"v1.39.0", false, false},
		{"1.39.0-beta.1", true, true},
		{"v1.39.0-beta.1", true, true},
		{"1.39.0-rc.2+build", true, true},
		{"1.39.0+build", false, false},
		{"1.39.0-", false, false},
		{"1.39.0-+build", false, false},
		{"1.39-beta.1", false, false},
		{"-beta.1", false, false},
		{"Dev", false, false},
		{"garbage", false, false},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.prerelease, isPrerelease(test.version))
			assert.Equal(t, test.enabled, FeatureEnabled(test.version))
		})
	}
}

func TestEnginesSatisfied(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		cli        string
		want       bool
	}{
		{"meets a lower bound", ">=1.38.0", "1.38.0", true},
		{"is below a lower bound", ">=1.38.0", "1.37.2", false},
		{"meets a lower bound on a beta build", ">=1.39.0", "1.39.0-beta.4", true},
		{"is below the lower bound on a beta of an older core", ">=1.39.0", "1.38.9-beta.1", false},
		{"every comparator must match", ">=1.39.0 <2.0.0", "2.0.0", false},
		{"inside a range", ">=1.39.0 <2.0.0", "1.99.99", true},
		{"strictly greater", ">1.39.0", "1.39.0", false},
		{"strictly less", "<1.39.0", "1.38.99", true},
		{"at most", "<=1.39.0", "1.39.0", true},
		{"exact", "=1.39.0", "1.39.1", false},
		{"a dev build skips the check", ">=99.0.0", "dev", true},
		{"an empty version skips the check", ">=99.0.0", "", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := EnginesSatisfied(test.constraint, test.cli)

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}

	t.Run("rejects a constraint that does not follow the grammar, also for a dev build", func(t *testing.T) {
		for _, constraint := range []string{"", "1.39.0", "^1.39.0", ">=1.39", ">=1.39.0-beta.1", ">=1.39.0  <2.0.0", ">= 1.39.0", ">=1.39.0 ", "~>1.0.0", ">=+1.0.0", ">=-1.0.0", ">=1.-1.0", ">=1_0.0.0", ">=01.0.0", ">=1..0", ">=99999999999999999999.0.0"} {
			for _, cli := range []string{"1.40.0", "dev"} {
				got, err := EnginesSatisfied(constraint, cli)

				require.ErrorIs(t, err, errInvalidConstraint, "constraint %q for %q", constraint, cli)
				assert.False(t, got)
			}
		}
	})
}
