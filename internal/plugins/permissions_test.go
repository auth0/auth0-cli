package plugins

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenPermissionsHash pins the hash of testdata/valid-manifest.json. If this
// changes, every consent that users have already granted is invalidated, so
// only update it on purpose.
const goldenPermissionsHash = "sha256:4051f9072e71ee6925a0e05870c9345bb1f8534b216f86a3969b8750a6350b07"

// encodeWithoutHTMLEscaping encodes value the way encoding/json does when HTML
// escaping is turned off, without the trailing newline.
func encodeWithoutHTMLEscaping(t *testing.T, value any) []byte {
	t.Helper()

	var buffer bytes.Buffer

	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	require.NoError(t, encoder.Encode(value))

	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
}

func TestManifest_Permissions(t *testing.T) {
	t.Run("returns non-nil empty slices when no command asks for permissions", func(t *testing.T) {
		got := parseMutated(t, removeCommand("permissions")).Permissions()

		assert.NotNil(t, got.Scopes)
		assert.NotNil(t, got.Hosts)
		assert.NotNil(t, got.Files)
		assert.NotNil(t, got.Env)
		assert.Empty(t, got.Scopes)
		assert.Empty(t, got.Hosts)
		assert.Empty(t, got.Files)
		assert.Empty(t, got.Env)
	})

	t.Run("merges, sorts and dedupes permissions across commands", func(t *testing.T) {
		manifest := parseMutated(t, setValue([]any{
			map[string]any{
				"name":        "one",
				"description": "One",
				"permissions": map[string]any{
					"scopes": []any{"read:users", "read:clients"},
					"hosts": []any{
						map[string]any{"host": "b.example.com", "reason": "B"},
						map[string]any{"host": "a.example.com", "reason": "Z"},
					},
					"files": []any{
						map[string]any{"path": "$HOME", "access": "write", "reason": "Cache"},
						map[string]any{"path": "$CWD", "access": "write", "reason": "Report"},
					},
					"env": []any{map[string]any{"name": "ZED", "reason": "Z"}},
				},
			},
			map[string]any{
				"name":        "two",
				"description": "Two",
				"permissions": map[string]any{
					"scopes": []any{"read:clients", "create:clients"},
					"hosts": []any{
						map[string]any{"host": "a.example.com", "reason": "A"},
						map[string]any{"host": "b.example.com", "reason": "B"},
					},
					"files": []any{map[string]any{"path": "$CWD", "access": "read", "reason": "Report"}},
					"env":   []any{map[string]any{"name": "ALPHA", "reason": "A"}},
				},
			},
		}, "commands"))

		want := Permissions{
			Scopes: []string{"create:clients", "read:clients", "read:users"},
			Hosts: []HostPermission{
				{Host: "a.example.com", Reason: "A"},
				{Host: "a.example.com", Reason: "Z"},
				{Host: "b.example.com", Reason: "B"},
			},
			Files: []FilePermission{
				{Path: "$CWD", Access: FileAccessRead, Reason: "Report"},
				{Path: "$CWD", Access: FileAccessWrite, Reason: "Report"},
				{Path: "$HOME", Access: FileAccessWrite, Reason: "Cache"},
			},
			Env: []EnvPermission{
				{Name: "ALPHA", Reason: "A"},
				{Name: "ZED", Reason: "Z"},
			},
		}

		assert.Equal(t, want, manifest.Permissions())
	})
}

func TestNormalizePermissions(t *testing.T) {
	t.Run("sorts, dedupes and never returns a nil slice", func(t *testing.T) {
		got := NormalizePermissions(Permissions{
			Scopes: []string{"b:x", "a:x", "b:x"},
			Hosts:  []HostPermission{{Host: "b.example.com", Reason: "B"}, {Host: "a.example.com", Reason: "A"}, {Host: "a.example.com", Reason: "A"}},
		})

		assert.Equal(t, []string{"a:x", "b:x"}, got.Scopes)
		assert.Equal(t, []HostPermission{{Host: "a.example.com", Reason: "A"}, {Host: "b.example.com", Reason: "B"}}, got.Hosts)
		assert.NotNil(t, got.Files)
		assert.NotNil(t, got.Env)
	})

	t.Run("does not change its argument", func(t *testing.T) {
		scopes := []string{"b:x", "a:x"}

		NormalizePermissions(Permissions{Scopes: scopes})

		assert.Equal(t, []string{"b:x", "a:x"}, scopes)
	})

	t.Run("is idempotent", func(t *testing.T) {
		once := NormalizePermissions(Permissions{Scopes: []string{"b:x", "a:x"}})

		assert.Equal(t, once, NormalizePermissions(once))
	})
}

func TestManifest_PermissionsHash(t *testing.T) {
	// The base manifest has two commands. Each case replaces it with a variant
	// and says whether the hash must stay the same.
	commandA := func() map[string]any {
		return map[string]any{
			"name":        "alpha",
			"description": "Alpha",
			"permissions": map[string]any{
				"scopes": []any{"read:clients", "read:users"},
				"hosts": []any{
					map[string]any{"host": "a.example.com", "reason": "A"},
					map[string]any{"host": "b.example.com", "reason": "B"},
				},
			},
		}
	}

	commandB := func() map[string]any {
		return map[string]any{
			"name":        "beta",
			"description": "Beta",
			"permissions": map[string]any{
				"scopes": []any{"read:users", "read:connections"},
				"env":    []any{map[string]any{"name": "HOME_DIR", "reason": "Find config"}},
			},
		}
	}

	hashOf := func(t *testing.T, commands ...map[string]any) string {
		t.Helper()

		list := make([]any, 0, len(commands))
		for _, command := range commands {
			list = append(list, command)
		}

		return parseMutated(t, setValue(list, "commands")).PermissionsHash()
	}

	// WithPermissions edits the permissions of a command built by commandA or commandB.
	withPermissions := func(command map[string]any, key string, value any) map[string]any {
		command["permissions"].(map[string]any)[key] = value
		return command
	}

	baseHash := hashOf(t, commandA(), commandB())

	t.Run("matches the golden hash for the example manifest", func(t *testing.T) {
		manifest, err := ParseManifest(readValidManifest(t))
		require.NoError(t, err)

		assert.Equal(t, goldenPermissionsHash, manifest.PermissionsHash())
	})

	tests := []struct {
		name     string
		variant  func() []map[string]any
		wantSame bool
	}{
		{
			name:     "is stable when commands are reordered",
			variant:  func() []map[string]any { return []map[string]any{commandB(), commandA()} },
			wantSame: true,
		},
		{
			name: "is stable when scopes and hosts are reordered",
			variant: func() []map[string]any {
				first := withPermissions(commandA(), "scopes", []any{"read:users", "read:clients"})
				first = withPermissions(first, "hosts", []any{
					map[string]any{"host": "b.example.com", "reason": "B"},
					map[string]any{"host": "a.example.com", "reason": "A"},
				})
				second := withPermissions(commandB(), "scopes", []any{"read:connections", "read:users"})

				return []map[string]any{second, first}
			},
			wantSame: true,
		},
		{
			name: "is stable when the same scope is requested by several commands",
			variant: func() []map[string]any {
				second := withPermissions(commandB(), "scopes", []any{"read:users", "read:connections", "read:clients"})

				return []map[string]any{commandA(), second}
			},
			wantSame: true,
		},
		{
			name: "changes when a scope is added",
			variant: func() []map[string]any {
				second := withPermissions(commandB(), "scopes", []any{"read:users", "read:connections", "delete:users"})

				return []map[string]any{commandA(), second}
			},
		},
		{
			name: "changes when only a host reason changes",
			variant: func() []map[string]any {
				first := withPermissions(commandA(), "hosts", []any{
					map[string]any{"host": "a.example.com", "reason": "A new reason"},
					map[string]any{"host": "b.example.com", "reason": "B"},
				})

				return []map[string]any{first, commandB()}
			},
		},
		{
			name: "changes when only an env reason changes",
			variant: func() []map[string]any {
				second := withPermissions(commandB(), "env", []any{map[string]any{"name": "HOME_DIR", "reason": "Something else"}})

				return []map[string]any{commandA(), second}
			},
		},
		{
			name: "changes when interactive flips on a command",
			variant: func() []map[string]any {
				first := commandA()
				first["interactive"] = true

				return []map[string]any{first, commandB()}
			},
		},
		{
			name: "is stable when only a description changes",
			variant: func() []map[string]any {
				first := commandA()
				first["description"] = "A completely different description"

				return []map[string]any{first, commandB()}
			},
			wantSame: true,
		},
		{
			name: "is stable when only a timeout changes",
			variant: func() []map[string]any {
				first := commandA()
				first["timeout"] = "45s"

				return []map[string]any{first, commandB()}
			},
			wantSame: true,
		},
		{
			name: "is stable when only flags or output change",
			variant: func() []map[string]any {
				first := commandA()
				first["flags"] = []any{map[string]any{"name": "verbose", "type": "bool", "description": "Be chatty"}}
				first["output"] = map[string]any{"json": true}

				return []map[string]any{first, commandB()}
			},
			wantSame: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hashOf(t, test.variant()...)

			if test.wantSame {
				assert.Equal(t, baseHash, got)
			} else {
				assert.NotEqual(t, baseHash, got)
			}
		})
	}

	t.Run("is stable when commands with different interactive values are reordered", func(t *testing.T) {
		interactive := commandA()
		interactive["interactive"] = true

		assert.Equal(t, hashOf(t, interactive, commandB()), hashOf(t, commandB(), interactive))
	})

	t.Run("changes when interactive flips on a command without permissions", func(t *testing.T) {
		plain := func(interactive bool) string {
			return hashOf(t, map[string]any{"name": "plain", "description": "Plain", "interactive": interactive})
		}

		assert.NotEqual(t, plain(false), plain(true))
	})

	t.Run("is stable when only the plugin version changes", func(t *testing.T) {
		manifest := parseMutated(t, combine(setValue("9.9.9", "version"), setValue([]any{commandA(), commandB()}, "commands")))

		assert.Equal(t, baseHash, manifest.PermissionsHash())
	})

	t.Run("hashes only the command entries when there are no permissions", func(t *testing.T) {
		manifest := parseMutated(t, removeCommand("permissions"))

		want := sha256.Sum256([]byte(`{"scopes":[],"hosts":[],"files":[],"env":[],"commands":[{"name":"checkmate","interactive":false}]}`))
		assert.Equal(t, hashPrefix+hex.EncodeToString(want[:]), manifest.PermissionsHash())
	})
}

// trickyStrings are strings that need escaping, or that encoding/json treats
// specially.
var trickyStrings = []string{
	"",
	"plain text",
	`say "hi"`,
	`back\slash`,
	"<b> & more",
	"tab\tnew\nline\rreturn",
	"backspace\bform\ffeed",
	"control\x00\x01\x1f",
	"delete\x7f",
	"c1 controls\u0080\u009b\u009f",
	"unicode é世界😀",
	"separators\u2028\u2029",
	"invalid utf8 \xff\xfe end",
}

func TestAppendJSONString(t *testing.T) {
	for i, value := range trickyStrings {
		t.Run(fmt.Sprintf("matches encoding/json for string %d", i), func(t *testing.T) {
			assert.Equal(t, string(encodeWithoutHTMLEscaping(t, value)), string(appendJSONString(nil, value)))
		})
	}
}

func TestManifest_PermissionsHash_MatchesEncodingJSON(t *testing.T) {
	manifest := &Manifest{
		Commands: []Command{
			{Name: "beta", Interactive: true},
			{
				Name: "alpha",
				Permissions: &Permissions{
					Scopes: []string{"read:clients"},
					Hosts:  []HostPermission{{Host: "example.com", Reason: trickyStrings[4]}},
					Files:  []FilePermission{{Path: trickyStrings[10], Access: FileAccessWrite, Reason: trickyStrings[11]}},
					Env:    []EnvPermission{{Name: "HOME", Reason: trickyStrings[6]}},
				},
			},
		},
	}

	type interactiveCommand struct {
		Name        string `json:"name"`
		Interactive bool   `json:"interactive"`
	}

	union := manifest.Permissions()
	reference := struct {
		Scopes   []string             `json:"scopes"`
		Hosts    []HostPermission     `json:"hosts"`
		Files    []FilePermission     `json:"files"`
		Env      []EnvPermission      `json:"env"`
		Commands []interactiveCommand `json:"commands"`
	}{
		Scopes:   union.Scopes,
		Hosts:    union.Hosts,
		Files:    union.Files,
		Env:      union.Env,
		Commands: []interactiveCommand{{Name: "alpha"}, {Name: "beta", Interactive: true}},
	}

	sum := sha256.Sum256(encodeWithoutHTMLEscaping(t, reference))

	assert.Equal(t, hashPrefix+hex.EncodeToString(sum[:]), manifest.PermissionsHash())
}

func TestManifest_PermissionsHash_Vectors(t *testing.T) {
	data, err := os.ReadFile("testdata/consent-hash-vectors.json")
	require.NoError(t, err)

	var file struct {
		Description string `json:"description"`
		Vectors     []struct {
			Name     string    `json:"name"`
			Commands []Command `json:"commands"`
			Hash     string    `json:"hash"`
		} `json:"vectors"`
	}
	require.NoError(t, json.Unmarshal(data, &file))

	assert.NotEmpty(t, file.Description)
	require.GreaterOrEqual(t, len(file.Vectors), 5)

	hashes := make(map[string]string, len(file.Vectors))

	for _, vector := range file.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			manifest := &Manifest{Commands: vector.Commands}

			assert.Equal(t, vector.Hash, manifest.PermissionsHash())
		})

		assert.NotContains(t, hashes, vector.Hash, "vectors %q and %q have the same hash", hashes[vector.Hash], vector.Name)
		hashes[vector.Hash] = vector.Name
	}

	t.Run("includes the golden hash of the example manifest", func(t *testing.T) {
		assert.Contains(t, hashes, goldenPermissionsHash)
	})
}
