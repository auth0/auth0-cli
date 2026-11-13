package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withAuthDisabled returns the golden file with an auth.disabled member added, or
// the golden file as it is when member is empty.
func withAuthDisabled(t *testing.T, member string) string {
	t.Helper()

	golden, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	if member == "" {
		return string(golden)
	}

	raw := strings.Replace(string(golden), `"issuer"`, member+`, "issuer"`, 1)
	require.NotEqual(t, string(golden), raw)

	return raw
}

func TestAuthDisabledPresence(t *testing.T) {
	tests := []struct {
		name         string
		member       string
		wantDisabled bool
		wantSet      bool
	}{
		{name: "true", member: `"disabled": true`, wantDisabled: true, wantSet: true},
		{name: "false", member: `"disabled": false`, wantSet: true},
		{name: "null counts as absent", member: `"disabled": null`},
	}

	for _, test := range tests {
		t.Run("read: "+test.name, func(t *testing.T) {
			cfg, err := Read(writeRaw(t, withAuthDisabled(t, test.member)))

			require.NoError(t, err)
			assert.Equal(t, test.wantDisabled, cfg.Auth.Disabled)
			assert.Equal(t, test.wantSet, cfg.Auth.DisabledSet)
		})

		t.Run("round trip: "+test.name, func(t *testing.T) {
			cfg, err := Read(writeRaw(t, withAuthDisabled(t, test.member)))
			require.NoError(t, err)

			out := filepath.Join(t.TempDir(), DefaultFileName)
			require.NoError(t, Write(out, cfg))

			written, err := os.ReadFile(out)
			require.NoError(t, err)

			if test.wantSet {
				assert.Contains(t, string(written), `"disabled": `+map[bool]string{true: "true", false: "false"}[test.wantDisabled])
			} else {
				assert.NotContains(t, string(written), `"disabled"`, "an absent member stays absent")
			}

			again, err := Read(out)
			require.NoError(t, err)
			assert.Equal(t, cfg.Auth, again.Auth)
		})
	}

	t.Run("a file without the member reads as not set", func(t *testing.T) {
		cfg, err := Read(writeRaw(t, withAuthDisabled(t, "")))

		require.NoError(t, err)
		assert.False(t, cfg.Auth.Disabled)
		assert.False(t, cfg.Auth.DisabledSet)
	})
}

func TestMergeParts(t *testing.T) {
	existing := func(authOff, authSet, webOff bool) *Config {
		cfg := exampleConfig()
		cfg.Auth = Auth{Disabled: authOff, DisabledSet: authSet}
		cfg.Web.Disabled = webOff
		cfg.Web.Unscoped, cfg.Web.UnscopedSet = true, true

		return cfg
	}

	tests := []struct {
		name         string
		existing     *Config
		authDisabled bool
		webDisabled  bool
		wantAuth     Auth
		wantWeb      Web
	}{
		{name: "no file, nothing off", wantAuth: Auth{}, wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(nil)}},
		{name: "no file, API off", authDisabled: true, wantAuth: Auth{Disabled: true, DisabledSet: true}, wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(nil)}},
		{name: "no file, web off", webDisabled: true, wantWeb: Web{Disabled: true}},
		{name: "file with an explicit false keeps it", existing: existing(false, true, false), wantAuth: Auth{DisabledSet: true}, wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(existing(false, true, false)), Unscoped: true, UnscopedSet: true}},
		{name: "auth.disabled in the file is cleared when the API part is on", existing: existing(true, true, false), wantAuth: Auth{}, wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(existing(true, true, false)), Unscoped: true, UnscopedSet: true}},
		{name: "API off turns an explicit false on", existing: existing(false, true, false), authDisabled: true, wantAuth: Auth{Disabled: true, DisabledSet: true}, wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(existing(false, true, false)), Unscoped: true, UnscopedSet: true}},
		{name: "web off drops the join address and web.unscoped", existing: existing(false, false, false), webDisabled: true, wantWeb: Web{Disabled: true}},
		{name: "web.disabled in the file is cleared when the web part is on", existing: existing(false, false, true), wantWeb: Web{BrowserSessionEndpoint: browserEndpoint(existing(false, false, true)), Unscoped: true, UnscopedSet: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := fullInputs()
			in.AuthDisabled, in.WebDisabled = test.authDisabled, test.webDisabled

			got, err := Merge(test.existing, derivedConfig(), in, MergeOptions{})

			require.NoError(t, err)
			assert.Equal(t, test.wantAuth, got.Auth)
			assert.Equal(t, test.wantWeb, got.Web)
		})
	}

	t.Run("a run with neither part on keeps the disabled values of the file", func(t *testing.T) {
		file := existing(true, true, true)
		in := Inputs{OrganizationName: "Shop", CompanyDomain: "shop.example"}

		got, err := Merge(file, derivedConfig(), in, MergeOptions{})

		require.NoError(t, err)
		assert.Equal(t, Auth{Disabled: true, DisabledSet: true}, got.Auth)
		assert.True(t, got.Web.Disabled)
	})

	t.Run("a part that is off in this run stays off next to one that is on", func(t *testing.T) {
		file := existing(true, true, true)
		in := fullInputs()
		in.AuthDisabled = true

		got, err := Merge(file, derivedConfig(), in, MergeOptions{})

		require.NoError(t, err)
		assert.Equal(t, Auth{Disabled: true, DisabledSet: true}, got.Auth)
		assert.False(t, got.Web.Disabled, "the web part is on")

		in = fullInputs()
		in.WebDisabled = true

		got, err = Merge(file, derivedConfig(), in, MergeOptions{})

		require.NoError(t, err)
		assert.Equal(t, Web{Disabled: true}, got.Web)
		assert.False(t, got.Auth.Disabled, "the API part is on")
	})

	t.Run("does not modify the existing file", func(t *testing.T) {
		file := existing(false, true, false)
		in := fullInputs()
		in.AuthDisabled, in.WebDisabled = true, true

		_, err := Merge(file, derivedConfig(), in, MergeOptions{})

		require.NoError(t, err)
		assert.Equal(t, existing(false, true, false), file)
	})
}

// browserEndpoint returns the join address a merge produces: the existing file's
// when it has one, else the one the inputs and the derived config give.
func browserEndpoint(existing *Config) string {
	if existing != nil {
		return existing.Web.BrowserSessionEndpoint
	}

	got, err := Merge(nil, derivedConfig(), fullInputs(), MergeOptions{})
	if err != nil {
		panic(err)
	}

	return got.Web.BrowserSessionEndpoint
}

func TestDiffAuthDisabled(t *testing.T) {
	cfg := func(disabled, set bool) *Config {
		c := exampleConfig()
		c.Auth = Auth{Disabled: disabled, DisabledSet: set}

		return c
	}

	tests := []struct {
		name string
		a, b *Config
		want []Change
	}{
		{name: "absent to true", a: cfg(false, false), b: cfg(true, true),
			want: []Change{{Path: "auth.disabled", Kind: ChangeAdded, New: "true"}}},
		{name: "absent to false is a change", a: cfg(false, false), b: cfg(false, true),
			want: []Change{{Path: "auth.disabled", Kind: ChangeAdded, New: "false"}}},
		{name: "false to true", a: cfg(false, true), b: cfg(true, true),
			want: []Change{{Path: "auth.disabled", Kind: ChangeChanged, Old: "false", New: "true"}}},
		{name: "true to false", a: cfg(true, true), b: cfg(false, true),
			want: []Change{{Path: "auth.disabled", Kind: ChangeChanged, Old: "true", New: "false"}}},
		{name: "true to true is none", a: cfg(true, true), b: cfg(true, true), want: []Change{}},
		{name: "absent to absent is none", a: cfg(false, false), b: cfg(false, false), want: []Change{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Diff(test.a, test.b))
		})
	}
}
