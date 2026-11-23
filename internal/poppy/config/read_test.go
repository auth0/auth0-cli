package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRaw(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), DefaultFileName)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRead(t *testing.T) {
	golden, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nope.json")

		_, err := Read(path)
		require.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrNotExist))

		_, err = ReadLenient(path)
		assert.True(t, errors.Is(err, fs.ErrNotExist))
	})

	t.Run("bad JSON names the file and the position", func(t *testing.T) {
		path := writeRaw(t, "{\n  \"protocol_version\": \"0.3\",\n  oops\n}\n")

		_, err := Read(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
		assert.Contains(t, err.Error(), "line 3")
	})

	t.Run("wrong type names the position", func(t *testing.T) {
		path := writeRaw(t, "{\n  \"protocol_version\": 3\n}\n")

		_, err := Read(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
		assert.Contains(t, err.Error(), "wrong type at line 2")
	})

	t.Run("empty file", func(t *testing.T) {
		_, err := Read(writeRaw(t, ""))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("content after the document", func(t *testing.T) {
		_, err := Read(writeRaw(t, string(golden)+"{}"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected content")
	})

	t.Run("unknown members are kept, not rejected", func(t *testing.T) {
		path := writeRaw(t, strings.Replace(string(golden), `"web"`, `"unknown": {"a": 1}, "web"`, 1))

		cfg, err := Read(path)
		require.NoError(t, err)

		got, err := marshal(cfg)
		require.NoError(t, err)
		assert.Contains(t, string(got), `"unknown": {`)
	})

	t.Run("unknown members of a scope hint survive a read, a merge and a write", func(t *testing.T) {
		raw := `{"protocol_version":"0.4","organization":{"name":"Shop","domain":"shop.example"},` +
			`"auth":{"issuer":"https://auth.shop.example/","scopes":{"checkout":"Pay"},` +
			`"scope_hints":{"checkout":{"changes":"none","personal_data":true,"x-note":"keep"}}},` +
			`"apis":[{"type":"openapi","url":"https://api.shop.example/o.json","description":"Orders"}]}`

		cfg, err := Read(writeRaw(t, raw))
		require.NoError(t, err)

		derived := &Config{Issuer: "https://auth.shop.example/", Scopes: map[string]Scope{"checkout": {Description: "Pay"}}}

		// Once with the hint kept as it is and once replaced by a flag.
		for _, in := range []Inputs{{}, {Hints: map[string]Hints{"checkout": {Changes: "reversible", PersonalData: true}}}} {
			merged, err := Merge(cfg, derived, in, MergeOptions{})
			require.NoError(t, err)

			path := filepath.Join(t.TempDir(), DefaultFileName)
			require.NoError(t, Write(path, merged))

			content, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(content), `"x-note": "keep"`)

			again, err := Read(path)
			require.NoError(t, err)
			assert.Equal(t, merged.Scopes["checkout"].Hints.Changes, again.Scopes["checkout"].Hints.Changes)
		}

		got, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.Contains(t, string(got), `"scope_hints":{"checkout":{"changes":"none","personal_data":true,"x-note":"keep"}}`)
	})

	t.Run("violations", func(t *testing.T) {
		path := writeRaw(t, strings.Replace(string(golden), `"shop.example"`, `"https://shop.example"`, 1))

		cfg, err := Read(path)
		assert.Nil(t, cfg)

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, []string{"organization.domain"}, paths(verr.Violations))
	})

	for _, version := range []string{"0.3", "0.4", "1.1", "1.12"} {
		t.Run("a file written at protocol "+version+" is still read", func(t *testing.T) {
			cfg, err := Read(writeRaw(t, strings.Replace(string(golden), `"protocol_version": "1.0"`, `"protocol_version": "`+version+`"`, 1)))

			require.NoError(t, err)
			assert.Equal(t, version, cfg.ProtocolVersion)
		})
	}

	t.Run("a protocol version outside the supported ones names them", func(t *testing.T) {
		_, err := Read(writeRaw(t, strings.Replace(string(golden), `"protocol_version": "1.0"`, `"protocol_version": "2.0"`, 1)))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `protocol_version must be one of "0.3", "0.4" or 1.x, got "2.0"`)
	})

	t.Run("web.unscoped and web.disabled are read", func(t *testing.T) {
		cfg, err := Read(writeRaw(t, strings.Replace(string(golden), `"browser_session_endpoint"`, `"unscoped": true, "disabled": true, "browser_session_endpoint"`, 1)))

		require.NoError(t, err)
		assert.True(t, cfg.Web.Unscoped)
		assert.True(t, cfg.Web.Disabled)
	})

	t.Run("a hint without personal_data is a violation", func(t *testing.T) {
		raw := strings.Replace(string(golden), `"changes": "irreversible",
        "personal_data": true`, `"changes": "irreversible"`, 1)
		require.NotEqual(t, string(golden), raw)

		_, err := Read(writeRaw(t, raw))

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, []string{"auth.scope_hints.checkout.personal_data"}, paths(verr.Violations))
		assert.Equal(t, "is required", verr.Violations[0].Message)
	})

	t.Run("a hint without changes is a violation", func(t *testing.T) {
		raw := strings.Replace(string(golden), `"changes": "irreversible",
        "personal_data": true`, `"personal_data": true`, 1)
		require.NotEqual(t, string(golden), raw)

		_, err := Read(writeRaw(t, raw))

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, []string{"auth.scope_hints.checkout.changes"}, paths(verr.Violations))
	})

	t.Run("a scope hint without a scope is a violation", func(t *testing.T) {
		raw := strings.Replace(string(golden), `"scope_hints": {`, `"scope_hints": {
      "nope": {"changes": "none", "personal_data": false},`, 1)

		_, err := Read(writeRaw(t, raw))

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, []string{"auth.scope_hints.nope"}, paths(verr.Violations))
	})

	t.Run("a document with none of agent, apis and web is a violation with a FIX", func(t *testing.T) {
		_, err := Read(writeRaw(t, `{"protocol_version":"0.4","organization":{"name":"Shop","domain":"shop.example"}}`))

		var verr *ValidationError
		require.True(t, errors.As(err, &verr))
		assert.Equal(t, []string{rootPath}, paths(verr.Violations))
		assert.NotContains(t, verr.Violations[0].Message, "FIX")
		assert.Contains(t, verr.Violations[0].Fix, "auth0 poppy setup --api")
		assert.Contains(t, verr.Error(), "\n  FIX  ")
	})

	t.Run("an api type the protocol does not define is read and accepted", func(t *testing.T) {
		cfg, err := Read(writeRaw(t, strings.Replace(string(golden), `"type": "openapi"`, `"type": "graphql"`, 1)))

		require.NoError(t, err)
		assert.Equal(t, "graphql", cfg.APIs[0].Type)
	})

	t.Run("lenient read skips validation", func(t *testing.T) {
		path := writeRaw(t, `{"protocol_version": "0.1"}`)

		cfg, err := ReadLenient(path)
		require.NoError(t, err)
		assert.Equal(t, "0.1", cfg.ProtocolVersion)
	})

	t.Run("lenient read still rejects bad JSON", func(t *testing.T) {
		_, err := ReadLenient(writeRaw(t, "{"))
		require.Error(t, err)
	})
}

func TestPosition(t *testing.T) {
	data := []byte("ab\ncd\nef")

	line, col := position(data, 0)
	assert.Equal(t, []int{1, 1}, []int{line, col})

	line, col = position(data, 4)
	assert.Equal(t, []int{2, 2}, []int{line, col})

	line, col = position(data, 1000)
	assert.Equal(t, []int{3, 3}, []int{line, col})
}
