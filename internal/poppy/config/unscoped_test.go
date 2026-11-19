package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

// withUnscoped returns the golden file with a web.unscoped member added, or the
// golden file as it is when member is empty.
func withUnscoped(t *testing.T, member string) string {
	t.Helper()

	golden, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	if member == "" {
		return string(golden)
	}

	raw := strings.Replace(string(golden), `"browser_session_endpoint"`, member+`, "browser_session_endpoint"`, 1)
	require.NotEqual(t, string(golden), raw)

	return raw
}

func TestWebUnscopedPresence(t *testing.T) {
	tests := []struct {
		name        string
		member      string
		wantUnscope bool
		wantSet     bool
	}{
		{name: "absent", member: ""},
		{name: "true", member: `"unscoped": true`, wantUnscope: true, wantSet: true},
		{name: "false", member: `"unscoped": false`, wantSet: true},
		{name: "null counts as absent", member: `"unscoped": null`},
	}

	for _, test := range tests {
		t.Run("read: "+test.name, func(t *testing.T) {
			cfg, err := Read(writeRaw(t, withUnscoped(t, test.member)))

			require.NoError(t, err)
			assert.Equal(t, test.wantUnscope, cfg.Web.Unscoped)
			assert.Equal(t, test.wantSet, cfg.Web.UnscopedSet)
		})

		t.Run("round trip: "+test.name, func(t *testing.T) {
			path := writeRaw(t, withUnscoped(t, test.member))

			cfg, err := Read(path)
			require.NoError(t, err)

			out := filepath.Join(t.TempDir(), DefaultFileName)
			require.NoError(t, Write(out, cfg))

			written, err := os.ReadFile(out)
			require.NoError(t, err)

			switch {
			case test.wantSet && test.wantUnscope:
				assert.Contains(t, string(written), `"unscoped": true`)
			case test.wantSet:
				assert.Contains(t, string(written), `"unscoped": false`, "an explicit false is written back")
			default:
				assert.NotContains(t, string(written), "unscoped", "an absent member stays absent")
			}

			again, err := Read(out)
			require.NoError(t, err)
			assert.Equal(t, cfg.Web, again.Web)
		})
	}
}

func TestMarshalUnscoped(t *testing.T) {
	tests := []struct {
		name string
		web  Web
		want string
	}{
		{name: "neither", web: Web{BrowserSessionEndpoint: "https://auth.shop.example/poppy/W/browser-session"}},
		{name: "set to false", web: Web{UnscopedSet: true}, want: `"unscoped": false`},
		{name: "set to true", web: Web{Unscoped: true, UnscopedSet: true}, want: `"unscoped": true`},
		{name: "true without the set flag is still written", web: Web{Unscoped: true}, want: `"unscoped": true`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := exampleConfig()
			cfg.Web = test.web

			got, err := marshal(cfg)
			require.NoError(t, err)

			if test.want == "" {
				assert.NotContains(t, string(got), "unscoped")
				return
			}

			assert.Contains(t, string(got), test.want)
		})
	}
}

func TestMergeWebUnscoped(t *testing.T) {
	existing := func(unscoped, set bool) *Config {
		cfg := exampleConfig()
		cfg.Web.Unscoped, cfg.Web.UnscopedSet = unscoped, set

		return cfg
	}

	tests := []struct {
		name        string
		existing    *Config
		input       *bool
		wantUnscope bool
		wantSet     bool
	}{
		{name: "no file, no input", existing: nil},
		{name: "no file, input true", existing: nil, input: boolPtr(true), wantUnscope: true, wantSet: true},
		{name: "no file, input false", existing: nil, input: boolPtr(false), wantSet: true},
		{name: "absent in the file, no input stays absent", existing: existing(false, false)},
		{name: "absent in the file, input false", existing: existing(false, false), input: boolPtr(false), wantSet: true},
		{name: "false in the file is kept", existing: existing(false, true), wantSet: true},
		{name: "true in the file is kept", existing: existing(true, true), wantUnscope: true, wantSet: true},
		{name: "input true overrides false", existing: existing(false, true), input: boolPtr(true), wantUnscope: true, wantSet: true},
		{name: "input false overrides true", existing: existing(true, true), input: boolPtr(false), wantSet: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := fullInputs()
			in.WebUnscoped = test.input

			got, err := Merge(test.existing, derivedConfig(), in, MergeOptions{})

			require.NoError(t, err)
			assert.Equal(t, test.wantUnscope, got.Web.Unscoped)
			assert.Equal(t, test.wantSet, got.Web.UnscopedSet)
		})
	}

	t.Run("does not modify the existing file", func(t *testing.T) {
		file := existing(false, false)
		in := fullInputs()
		in.WebUnscoped = boolPtr(true)

		_, err := Merge(file, derivedConfig(), in, MergeOptions{})

		require.NoError(t, err)
		assert.Equal(t, existing(false, false), file)
	})
}

func TestDiffWebUnscoped(t *testing.T) {
	cfg := func(unscoped, set bool) *Config {
		c := exampleConfig()
		c.Web.Unscoped, c.Web.UnscopedSet = unscoped, set

		return c
	}

	tests := []struct {
		name string
		a, b *Config
		want []Change
	}{
		{name: "absent to false is a change", a: cfg(false, false), b: cfg(false, true),
			want: []Change{{Path: "web.unscoped", Kind: ChangeAdded, New: "false"}}},
		{name: "absent to true", a: cfg(false, false), b: cfg(true, true),
			want: []Change{{Path: "web.unscoped", Kind: ChangeAdded, New: "true"}}},
		{name: "false to true", a: cfg(false, true), b: cfg(true, true),
			want: []Change{{Path: "web.unscoped", Kind: ChangeChanged, Old: "false", New: "true"}}},
		{name: "true to false", a: cfg(true, true), b: cfg(false, true),
			want: []Change{{Path: "web.unscoped", Kind: ChangeChanged, Old: "true", New: "false"}}},
		{name: "false to false is none", a: cfg(false, true), b: cfg(false, true), want: []Change{}},
		{name: "absent to absent is none", a: cfg(false, false), b: cfg(false, false), want: []Change{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Diff(test.a, test.b))
		})
	}
}
