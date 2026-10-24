package devlink

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
)

const testID = "auth0/checkmate"

// newLinks returns the links of a plugins directory that does not exist yet.
func newLinks(t *testing.T) Links {
	t.Helper()

	return Links{Dir: filepath.Join(t.TempDir(), "plugins")}
}

// abs returns an absolute path for a name, the way the commands make one.
func abs(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(t.TempDir(), name)
}

func (l Links) read(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(l.path())
	require.NoError(t, err)

	return string(data)
}

func (l Links) writeRaw(t *testing.T, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(l.Dir, 0o700))
	require.NoError(t, os.WriteFile(l.path(), []byte(content), 0o600))
}

// build writes a build of the plugin in a directory: its manifest and an
// executable, and returns the link to it.
func build(t *testing.T, version string) Link {
	t.Helper()

	dir := t.TempDir()
	writeManifest(t, dir, version)

	bin := DefaultBin(dir, "checkmate")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))

	return Link{Path: dir, Bin: bin}
}

func writeManifest(t *testing.T, dir, version string) {
	t.Helper()

	data, err := json.Marshal(plugins.Manifest{
		ID:          testID,
		Version:     version,
		APIVersion:  1,
		Description: "Checks the tenant",
		Owner:       plugins.Owner{Team: "Test team", Bugs: "https://example.com/bugs"},
		Platforms:   []plugins.Platform{plugins.PlatformLinuxX64},
		Commands:    []plugins.Command{{Name: "checkmate", Description: "Check a tenant"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestName), data, 0o600))
}

func TestLinks_LoadIsRefusedWhenTheFileIsNotTrustworthy(t *testing.T) {
	path := filepath.Join(os.TempDir(), "a", "plugin")
	if runtime.GOOS == "windows" {
		path = `C:\a\plugin`
	}

	link := `{"path":` + strconv.Quote(path) + `,"bin":` + strconv.Quote(path) + `}`

	tests := []struct {
		name    string
		content string
	}{
		{"not JSON", `{"formatVersion":`},
		{"empty", ``},
		{"a duplicate key", `{"formatVersion":1,"links":{},"links":{}}`},
		{"a duplicate key in a link", `{"formatVersion":1,"links":{"auth0/a":{"path":` + strconv.Quote(path) + `,"path":` + strconv.Quote(path) + `,"bin":` + strconv.Quote(path) + `}}}`},
		{"a key in another case", `{"formatVersion":1,"links":{},"Links":{"auth0/a":` + link + `}}`},
		{"a name of a field in another case", `{"formatVersion":1,"Links":{"auth0/a":` + link + `}}`},
		{"a field it does not know", `{"formatVersion":1,"links":{},"extra":true}`},
		{"a field of a link it does not know", `{"formatVersion":1,"links":{"auth0/a":{"path":` + strconv.Quote(path) + `,"bin":` + strconv.Quote(path) + `,"extra":1}}}`},
		{"no links field", `{"formatVersion":1}`},
		{"no version", `{"links":{}}`},
		{"a version it does not read", `{"formatVersion":2,"links":{}}`},
		{"a link without bin", `{"formatVersion":1,"links":{"auth0/a":{"path":` + strconv.Quote(path) + `}}}`},
		{"a path that is not absolute", `{"formatVersion":1,"links":{"auth0/a":{"path":"relative/dir","bin":` + strconv.Quote(path) + `}}}`},
		{"a bin that is not absolute", `{"formatVersion":1,"links":{"auth0/a":{"path":` + strconv.Quote(path) + `,"bin":"checkmate"}}}`},
		{"an empty plugin ID", `{"formatVersion":1,"links":{"":` + link + `}}`},
		{"a file larger than allowed", `{"formatVersion":1,"links":{},"pad":"` + strings.Repeat("x", maxFileBytes) + `"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			links := newLinks(t)
			links.writeRaw(t, test.content)

			_, err := links.Load()
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrDamaged)
			assert.Contains(t, err.Error(), links.path(), "the error names the file")

			// A damaged file is never replaced.
			require.Error(t, links.Add(context.Background(), testID, Link{Path: abs(t, "a"), Bin: abs(t, "b")}))
			assert.Equal(t, test.content, links.read(t))
		})
	}

	t.Run("a directory", func(t *testing.T) {
		links := newLinks(t)
		require.NoError(t, os.MkdirAll(links.path(), 0o700))

		_, err := links.Load()
		assert.ErrorIs(t, err, ErrDamaged)
	})
}

func TestLinks_LoadOfAValidFile(t *testing.T) {
	t.Run("a missing file is no links, and nothing is created", func(t *testing.T) {
		links := newLinks(t)

		got, err := links.Load()
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.NoDirExists(t, links.Dir)
	})

	t.Run("an empty list of links", func(t *testing.T) {
		links := newLinks(t)
		links.writeRaw(t, `{"formatVersion":1,"links":{}}`)

		got, err := links.Load()
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a null list of links", func(t *testing.T) {
		links := newLinks(t)
		links.writeRaw(t, `{"formatVersion":1,"links":null}`)

		got, err := links.Load()
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestLinks_AddRemove(t *testing.T) {
	ctx := context.Background()
	links := newLinks(t)

	first := Link{Path: abs(t, "one"), Bin: abs(t, "one/checkmate")}
	second := Link{Path: abs(t, "two"), Bin: abs(t, "two/checkmate")}
	other := Link{Path: abs(t, "three"), Bin: abs(t, "three/other")}

	require.NoError(t, links.Add(ctx, testID, first))
	require.NoError(t, links.Add(ctx, "auth0/other", other))

	got, err := links.Load()
	require.NoError(t, err)
	assert.Equal(t, map[string]Link{testID: first, "auth0/other": other}, got)

	t.Run("linking again replaces the link of that ID only", func(t *testing.T) {
		require.NoError(t, links.Add(ctx, testID, second))

		all, err := links.Load()
		require.NoError(t, err)
		assert.Equal(t, map[string]Link{testID: second, "auth0/other": other}, all)
	})

	t.Run("the file is private, and is valid JSON with the version", func(t *testing.T) {
		var document map[string]any
		require.NoError(t, json.Unmarshal([]byte(links.read(t)), &document))
		assert.EqualValues(t, 1, document["formatVersion"])

		if runtime.GOOS != "windows" {
			info, err := os.Stat(links.path())
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			info, err = os.Stat(links.Dir)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
		}
	})

	t.Run("removing one leaves the other", func(t *testing.T) {
		require.NoError(t, links.Remove(ctx, testID))

		all, err := links.Load()
		require.NoError(t, err)
		assert.Equal(t, map[string]Link{"auth0/other": other}, all)
	})

	t.Run("an unknown plugin is not linked, and the file is untouched", func(t *testing.T) {
		before := links.read(t)

		err := links.Remove(ctx, testID)
		require.ErrorIs(t, err, ErrNotLinked)
		assert.Contains(t, err.Error(), testID)
		assert.Equal(t, before, links.read(t))
	})

	t.Run("removing the last link leaves a valid empty file", func(t *testing.T) {
		require.NoError(t, links.Remove(ctx, "auth0/other"))

		got, err := links.Load()
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.JSONEq(t, `{"formatVersion":1,"links":{}}`, links.read(t))
	})

	t.Run("no temporary file is left behind", func(t *testing.T) {
		entries, err := os.ReadDir(links.Dir)
		require.NoError(t, err)

		for _, entry := range entries {
			assert.Contains(t, []string{FileName, lockName}, entry.Name())
		}
	})
}

func TestLinks_ConcurrentChangesAreNotLost(t *testing.T) {
	links := newLinks(t)
	base := t.TempDir()

	var group sync.WaitGroup

	errs := make(chan error, 20)

	for n := range 20 {
		group.Add(1)

		go func() {
			defer group.Done()

			name := "plugin" + strconv.Itoa(n)
			errs <- links.Add(context.Background(), "auth0/"+name, Link{Path: filepath.Join(base, name), Bin: filepath.Join(base, name, name)})
		}()
	}

	group.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	got, err := links.Load()
	require.NoError(t, err)
	assert.Len(t, got, 20, "every change is kept")
}

func TestLinks_AddRefusesWhatIsNotAbsolute(t *testing.T) {
	tests := []struct {
		name string
		id   string
		link Link
	}{
		{"a relative path", testID, Link{Path: "dir", Bin: abs(t, "bin")}},
		{"a relative bin", testID, Link{Path: abs(t, "dir"), Bin: "bin"}},
		{"no ID", "", Link{Path: abs(t, "dir"), Bin: abs(t, "bin")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			links := newLinks(t)

			require.Error(t, links.Add(context.Background(), test.id, test.link))
			assert.NoDirExists(t, links.Dir)
		})
	}
}

func TestLinks_AFailedWriteLeavesTheOldFile(t *testing.T) {
	ctx := context.Background()
	links := newLinks(t)
	first := Link{Path: abs(t, "one"), Bin: abs(t, "one/checkmate")}

	require.NoError(t, links.Add(ctx, testID, first))

	before := links.read(t)
	failure := errors.New("disk is read only")

	original := renameFile
	renameFile = func(string, string) error { return failure }

	t.Cleanup(func() { renameFile = original })

	err := links.Add(ctx, "auth0/other", Link{Path: abs(t, "two"), Bin: abs(t, "two/other")})
	require.ErrorIs(t, err, failure)
	assert.Contains(t, err.Error(), links.path())

	err = links.Remove(ctx, testID)
	require.ErrorIs(t, err, failure)

	assert.Equal(t, before, links.read(t), "readers still see the old file")

	entries, err := os.ReadDir(links.Dir)
	require.NoError(t, err)

	for _, entry := range entries {
		assert.Contains(t, []string{FileName, lockName}, entry.Name(), "no staged file is left behind")
	}
}

func TestLink_Open(t *testing.T) {
	t.Run("reads the build as it is on disk now", func(t *testing.T) {
		link := build(t, "1.0.0")

		opened, err := link.Open(testID)
		require.NoError(t, err)
		assert.Equal(t, link.Path, opened.Dir)
		assert.Equal(t, link.Bin, opened.Bin)
		assert.Equal(t, filepath.Join(link.Path, ManifestName), opened.ManifestPath)
		assert.Equal(t, "1.0.0", opened.Manifest.Version)

		writeManifest(t, link.Path, "1.1.0")

		opened, err = link.Open(testID)
		require.NoError(t, err)
		assert.Equal(t, "1.1.0", opened.Manifest.Version, "an edit shows without linking again")
	})

	tests := []struct {
		name    string
		change  func(t *testing.T, link Link)
		id      string
		wantErr string
	}{
		{
			name:    "the manifest has a problem",
			change:  func(t *testing.T, link Link) { writeManifestText(t, link, `{"id":"auth0/checkmate"}`) },
			wantErr: "required field is missing",
		},
		{
			name:    "the manifest is not JSON",
			change:  func(t *testing.T, link Link) { writeManifestText(t, link, `nope`) },
			wantErr: "not valid JSON",
		},
		{
			name:    "the manifest is gone",
			change:  func(t *testing.T, link Link) { require.NoError(t, os.Remove(filepath.Join(link.Path, ManifestName))) },
			wantErr: ManifestName,
		},
		{
			name:    "the manifest is for another plugin",
			id:      "auth0/other",
			wantErr: "is for the plugin auth0/checkmate, not auth0/other",
		},
		{
			name:    "the manifest is a directory",
			change:  func(t *testing.T, link Link) { replaceWithDirectory(t, filepath.Join(link.Path, ManifestName)) },
			wantErr: "not a regular file",
		},
		{
			name:    "the manifest is larger than allowed",
			change:  func(t *testing.T, link Link) { writeManifestText(t, link, strings.Repeat(" ", maxFileBytes+1)) },
			wantErr: "larger than the allowed size",
		},
		{
			name:    "the executable is gone",
			change:  func(t *testing.T, link Link) { require.NoError(t, os.Remove(link.Bin)) },
			wantErr: "{bin}",
		},
		{
			name:    "the executable is a directory",
			change:  func(t *testing.T, link Link) { replaceWithDirectory(t, link.Bin) },
			wantErr: "is not a regular file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			link := build(t, "1.0.0")

			if test.change != nil {
				test.change(t, link)
			}

			id := testID
			if test.id != "" {
				id = test.id
			}

			_, err := link.Open(id)
			require.Error(t, err)

			assert.Contains(t, err.Error(), strings.ReplaceAll(test.wantErr, "{bin}", link.Bin))
		})
	}
}

func writeManifestText(t *testing.T, link Link, text string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(link.Path, ManifestName), []byte(text), 0o600))
}

func replaceWithDirectory(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o700))
}

func TestReadManifest_ReportsEveryProblemOfAnInvalidManifest(t *testing.T) {
	link := build(t, "1.0.0")
	writeManifestText(t, link, `{"id":"Bad","version":"1","apiVersion":1,"description":"d","owner":{"team":"t","bugs":"https://example.com"},"platforms":["linux-x64"],"commands":[{"name":"a","description":"d"}],"extra":true}`)

	_, err := ReadManifest(filepath.Join(link.Path, ManifestName))

	invalid, ok := errors.AsType[*plugins.ValidationError](err)
	require.True(t, ok, "a manifest with problems is a validation error, got %v", err)
	assert.Contains(t, strings.Join(invalid.Errors, "\n"), "extra: unknown field")
	assert.Contains(t, strings.Join(invalid.Errors, "\n"), "id:")
}

func TestCheckExecutable(t *testing.T) {
	dir := t.TempDir()

	regular := filepath.Join(dir, "regular")
	require.NoError(t, os.WriteFile(regular, []byte("x"), 0o755))

	t.Run("a regular file", func(t *testing.T) {
		require.NoError(t, CheckExecutable(regular))
	})

	t.Run("a missing file names the path", func(t *testing.T) {
		err := CheckExecutable(filepath.Join(dir, "missing"))
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.Contains(t, err.Error(), filepath.Join(dir, "missing"))
	})

	t.Run("a directory", func(t *testing.T) {
		err := CheckExecutable(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a regular file")
	})

	if runtime.GOOS == "windows" {
		return
	}

	t.Run("a file without the executable bit", func(t *testing.T) {
		plain := filepath.Join(dir, "plain")
		require.NoError(t, os.WriteFile(plain, []byte("x"), 0o644))

		err := CheckExecutable(plain)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "chmod +x "+plain)
	})

	t.Run("a link to a regular file is followed", func(t *testing.T) {
		link := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(regular, link))

		require.NoError(t, CheckExecutable(link))
	})

	t.Run("a link to a directory is refused", func(t *testing.T) {
		link := filepath.Join(dir, "dirlink")
		require.NoError(t, os.Symlink(dir, link))

		require.Error(t, CheckExecutable(link))
	})

	t.Run("a link to nothing is refused", func(t *testing.T) {
		link := filepath.Join(dir, "dangling")
		require.NoError(t, os.Symlink(filepath.Join(dir, "gone"), link))

		require.ErrorIs(t, CheckExecutable(link), os.ErrNotExist)
	})
}

func TestDefaultBin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "build")

	want := filepath.Join(dir, "checkmate")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}

	assert.Equal(t, want, DefaultBin(dir, "checkmate"))
}
