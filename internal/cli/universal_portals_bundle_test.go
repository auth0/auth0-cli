package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const universalPortalsSourceDir = "data/universal-portals/_source"

// universalPortalsSourceHash is a hash of everything the embedded file is built from: the lockfile,
// the build script and every file under src. The tool's own tests compute the same value, and the
// file records it in its first lines. Paths use "/" and are sorted, and line endings are LF, so
// the value is the same on every machine.
func universalPortalsSourceHash(t *testing.T, root string) string {
	t.Helper()

	files := []string{"package-lock.json", "scripts/bundle.mjs"}
	err := filepath.WalkDir(filepath.Join(root, "src"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, relErr := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(relative))
		return relErr
	})
	require.NoError(t, err)
	sort.Strings(files)

	hash := sha256.New()
	for _, file := range files {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		require.NoError(t, readErr)
		text := strings.ReplaceAll(string(content), "\r\n", "\n")
		hash.Write([]byte(file + "\x00" + text + "\x00"))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// This pins the definition above. The tool's tests check the same value, written out by hand, so
// the two implementations cannot drift apart.
func TestUniversalPortalsSourceHashDefinition(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"package-lock.json":  "{}",
		"scripts/bundle.mjs": "x",
		"src/a.ts":           "a",
		"src/deep/b.ts":      "b",
		"test/ignored.ts":    "not part of the hash",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0o644))
	}

	want := sha256.Sum256([]byte("package-lock.json\x00{}\x00scripts/bundle.mjs\x00x\x00src/a.ts\x00a\x00src/deep/b.ts\x00b\x00"))

	assert.Equal(t, hex.EncodeToString(want[:]), universalPortalsSourceHash(t, root))
}

// The binary ships the embedded file, so a file that is older than its sources would ship old code.
func TestEmbeddedUniversalPortalsScriptMatchesItsSources(t *testing.T) {
	if _, err := os.Stat(filepath.Join(universalPortalsSourceDir, "src")); errors.Is(err, fs.ErrNotExist) {
		t.Skip("the sources are not in this checkout")
	}

	head := string(universalPortalsScript)
	if len(head) > 600 {
		head = head[:600]
	}
	match := regexp.MustCompile(`(?m)^// source-sha256: ([0-9a-f]{64})$`).FindStringSubmatch(head)
	require.NotNil(t, match, "the embedded file does not start with its source hash")

	assert.Equal(t,
		match[1],
		universalPortalsSourceHash(t, universalPortalsSourceDir),
		`the sources changed after the embedded file was built: run "npm run bundle" in %s and commit the result`,
		universalPortalsSourceDir,
	)
}

// The plugin framework skips a plugin whose name matches a built-in command. Once the framework is on
// this branch, the embedded command must go, or the installed plugin is silently ignored.
func TestUniversalPortalsCommandIsRemovedWhenTheFrameworkArrives(t *testing.T) {
	_, err := os.Stat("plugins.go")

	require.True(t, errors.Is(err, fs.ErrNotExist),
		"plugins.go exists, so the plugin framework is on this branch. Remove the embedded universal-portals "+
			"command first: the list is at the top of universal_portals.go")
}
