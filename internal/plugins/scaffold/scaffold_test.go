package scaffold

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// goodOptions returns options that create a plugin in dir.
func goodOptions(dir string) Options {
	return Options{
		Name:               "demo",
		ID:                 "auth0/demo",
		Version:            "0.1.0",
		Description:        "A plugin for the Auth0 CLI",
		Command:            "demo",
		CommandDescription: "Lists the clients",
		Dir:                dir,
		Team:               "Platform team",
		Bugs:               "https://example.com/issues",
		License:            "UNLICENSED",
		Scopes:             []string{"read:clients"},
		Lang:               LangGo,
		CLIVersion:         "1.40.0",
	}
}

// miniTree is a small template tree with every kind of file the layout allows.
func miniTree() fstest.MapFS {
	return fstest.MapFS{
		"go/main.go.tmpl":      {Data: []byte("package main // __PLUGIN_NAME__ __COMMAND__ __LICENSE__\n")},
		"go/.gitignore.tmpl":   {Data: []byte("/__PLUGIN_NAME__\n")},
		"go/plain.txt":         {Data: []byte("kept ${{ secrets.TOKEN }} and __lower__ as they are\n")},
		"go/sub/dir/file.tmpl": {Data: []byte("nested\n")},
		"ts/package.json.tmpl": {Data: []byte(`{"name": "__PLUGIN_NAME__"}` + "\n")},
	}
}

// readProject returns the content of every file under dir by its slash separated
// path.
func readProject(t *testing.T, dir string) map[string]string {
	t.Helper()

	files := map[string]string{}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(relative)] = string(data)

		return nil
	})
	require.NoError(t, err)

	return files
}

// readManifest parses the plugin.json of a project with the strict parser.
func readManifest(t *testing.T, dir string) *plugins.Manifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	require.NoError(t, err)

	manifest, err := plugins.ParseManifest(data)
	require.NoError(t, err)

	return manifest
}

// entries returns the names in a directory.
func entries(t *testing.T, dir string) []string {
	t.Helper()

	list, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := []string{}
	for _, entry := range list {
		names = append(names, entry.Name())
	}

	return names
}

// requireInputError fails unless err is an InputError, the kind that the caller
// can fix.
func requireInputError(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)

	var input *InputError
	assert.ErrorAs(t, err, &input, "an error of the input")
}

// permissions returns the modes that a new directory and a new file get here:
// 0o755 and 0o644 minus the umask, which is read from a probe of each.
func permissions(t *testing.T) (dir, file os.FileMode) {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), nil, 0o644))

	dirInfo, err := os.Stat(filepath.Join(root, "dir"))
	require.NoError(t, err)

	fileInfo, err := os.Stat(filepath.Join(root, "file"))
	require.NoError(t, err)

	return dirInfo.Mode().Perm(), fileInfo.Mode().Perm()
}

func TestCreate_Embedded(t *testing.T) {
	for _, lang := range []Lang{LangGo, LangTypeScript} {
		t.Run(string(lang), func(t *testing.T) {
			opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
			opts.Lang = lang

			files, err := Create(opts, Embedded())
			require.NoError(t, err)

			assert.IsIncreasing(t, files, "the paths are sorted and unique")
			assert.Contains(t, files, "plugin.json")
			assert.Contains(t, files, "README.md")

			written := readProject(t, opts.Dir)
			assert.Len(t, written, len(files))

			for path, content := range written {
				assert.Contains(t, files, path)
				assert.NotContains(t, path, templateSuffix, "the suffix is dropped")
				assert.NotRegexp(t, leftoverToken, content, path)
				assert.NotContains(t, content, "__PLUGIN_NAME__", path)
			}

			assert.Contains(t, written["README.md"], "auth0 demo")

			if runtime.GOOS != "windows" {
				wantDir, wantFile := permissions(t)

				assert.NoError(t, filepath.WalkDir(opts.Dir, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					info, err := entry.Info()
					require.NoError(t, err)

					// The mode is 0o755 or 0o644 minus what the umask takes away. The
					// project directory is the exception: it is made private by the
					// temporary directory and set to 0o755 by a chmod, which the umask
					// does not touch.
					want := wantFile

					switch {
					case path == opts.Dir:
						want = 0o755
					case entry.IsDir():
						want = wantDir
					}

					assert.Equal(t, want, info.Mode().Perm(), path)

					return nil
				}))
			}
		})
	}
}

func TestCreate_Manifest(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Options)
		check  func(*testing.T, *plugins.Manifest)
	}{
		{
			name: "defaults",
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Equal(t, "auth0/demo", m.ID)
				assert.Equal(t, "0.1.0", m.Version)
				assert.Equal(t, 1, m.APIVersion)
				assert.Equal(t, plugins.Owner{Team: "Platform team", Bugs: "https://example.com/issues"}, m.Owner)
				assert.Equal(t, plugins.AllPlatforms(), m.Platforms)
				assert.Contains(t, m.Schema, "plugin.v1.schema.json")
				assert.Equal(t, ">=1.40.0", m.Engines.Auth0CLI)

				require.Len(t, m.Commands, 1)
				command := m.Commands[0]
				assert.Equal(t, "demo", command.Name)
				assert.Equal(t, "Lists the clients", command.Description)
				assert.False(t, command.Interactive)
				assert.Equal(t, "5m", command.Timeout)
				assert.Equal(t, &plugins.Output{JSON: true, AgentMode: true}, command.Output)
				assert.Equal(t, []string{"read:clients"}, command.Permissions.Scopes)
				assert.NotNil(t, m.RootCommand(), "the command is named like the plugin")
			},
		},
		{
			name:   "no scopes",
			change: func(o *Options) { o.Scopes = nil },
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Empty(t, m.Commands[0].Permissions.Scopes)
			},
		},
		{
			name:   "empty scopes",
			change: func(o *Options) { o.Scopes = []string{} },
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Empty(t, m.Commands[0].Permissions.Scopes)
			},
		},
		{
			name:   "several scopes",
			change: func(o *Options) { o.Scopes = []string{"read:clients", "read:users"} },
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Equal(t, []string{"read:clients", "read:users"}, m.Commands[0].Permissions.Scopes)
			},
		},
		{
			name:   "another command than the plugin",
			change: func(o *Options) { o.Command = "lister" },
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Equal(t, "lister", m.Commands[0].Name)
				assert.Nil(t, m.RootCommand())
			},
		},
		{
			name:   "a prerelease CLI requires its core version",
			change: func(o *Options) { o.CLIVersion = "v1.41.0-beta.2" },
			check:  func(t *testing.T, m *plugins.Manifest) { assert.Equal(t, ">=1.41.0", m.Engines.Auth0CLI) },
		},
		{
			name:   "a build with no version requires nothing",
			change: func(o *Options) { o.CLIVersion = "dev" },
			check:  func(t *testing.T, m *plugins.Manifest) { assert.Nil(t, m.Engines) },
		},
		{
			name:   "an empty version requires nothing",
			change: func(o *Options) { o.CLIVersion = "" },
			check:  func(t *testing.T, m *plugins.Manifest) { assert.Nil(t, m.Engines) },
		},
		{
			name: "free text with quotes, markup and a token",
			change: func(o *Options) {
				o.Description = `He said "hi" \ <b>&</b> __PLUGIN_NAME__ $(rm -rf /)`
				o.CommandDescription = "Two\nlines and a tab\t"
				o.Team = "Team \"A\"\nand B"
			},
			check: func(t *testing.T, m *plugins.Manifest) {
				assert.Equal(t, `He said "hi" \ <b>&</b> __PLUGIN_NAME__ $(rm -rf /)`, m.Description)
				assert.Equal(t, "Two\nlines and a tab\t", m.Commands[0].Description)
				assert.Equal(t, "Team \"A\"\nand B", m.Owner.Team)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
			if test.change != nil {
				test.change(&opts)
			}

			_, err := Create(opts, miniTree())
			require.NoError(t, err)

			test.check(t, readManifest(t, opts.Dir))
		})
	}

	t.Run("the file is indented by two spaces and ends with a newline", func(t *testing.T) {
		opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
		opts.Description = "Less < more & more"

		_, err := Create(opts, miniTree())
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(opts.Dir, "plugin.json"))
		require.NoError(t, err)

		assert.Contains(t, string(data), "\n  \"id\": \"auth0/demo\",\n")
		assert.Contains(t, string(data), "Less < more & more", "markup is not escaped")
		assert.True(t, strings.HasSuffix(string(data), "}\n"))
	})
}

func TestCreate_FreeTextStaysOutOfSourceFiles(t *testing.T) {
	for _, lang := range []Lang{LangGo, LangTypeScript} {
		t.Run(string(lang), func(t *testing.T) {
			opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
			opts.Lang = lang
			opts.Description = `HOSTILE-DESCRIPTION "; rm -rf / #`
			opts.CommandDescription = "HOSTILE-COMMAND\nnewline `backtick` ${x}"
			opts.Team = "HOSTILE-TEAM\"; drop"
			opts.Bugs = "https://example.com/HOSTILE-BUGS?a=\"b\""

			_, err := Create(opts, Embedded())
			require.NoError(t, err)

			for path, content := range readProject(t, opts.Dir) {
				if path == "plugin.json" {
					continue
				}

				assert.NotContains(t, content, "HOSTILE", path)
			}

			manifest := readManifest(t, opts.Dir)
			assert.Equal(t, opts.Description, manifest.Description)
			assert.Equal(t, opts.Team, manifest.Owner.Team)
		})
	}
}

func TestCreate_InvalidOptions(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Options)
		want   string
	}{
		{"no language", func(o *Options) { o.Lang = "" }, "language"},
		{"unknown language", func(o *Options) { o.Lang = "rust" }, `"rust" is not one of go and ts`},
		{"uppercase name", func(o *Options) { o.Name = "Demo" }, "name"},
		{"name that starts with a digit", func(o *Options) { o.Name = "1demo" }, "name"},
		{"name with an underscore", func(o *Options) { o.Name = "my_plugin" }, "name"},
		{"name that is a path", func(o *Options) { o.Name = "../demo" }, "name"},
		{"empty name", func(o *Options) { o.Name = "" }, "name"},
		{"ID that is not an ID", func(o *Options) { o.ID = "demo" }, "registry/name"},
		{"ID of another registry", func(o *Options) { o.ID = "acme/demo" }, "auth0/demo"},
		{"ID of another plugin", func(o *Options) { o.ID = "auth0/other" }, "auth0/demo"},
		{"ID with a Windows device name", func(o *Options) { o.Name, o.ID, o.Command = "con", "auth0/con", "con" }, "directory name"},
		{"uppercase command", func(o *Options) { o.Command = "Lister" }, "command"},
		{"command with a space", func(o *Options) { o.Command = "my command" }, "command"},
		{"license with a space", func(o *Options) { o.License = "MIT license" }, "license"},
		{"license that is code", func(o *Options) { o.License = "MIT$(id)" }, "license"},
		{"empty license", func(o *Options) { o.License = "" }, "license"},
		{"no directory", func(o *Options) { o.Dir = "" }, "directory is required"},
		{"no team", func(o *Options) { o.Team = "" }, "team is required"},
		{"no bugs URL", func(o *Options) { o.Bugs = "" }, "bugs URL is required"},
		{"bugs URL without TLS", func(o *Options) { o.Bugs = "http://example.com/issues" }, "owner.bugs"},
		{"version with a prefix", func(o *Options) { o.Version = "v1.0.0" }, "version"},
		{"description of two lines", func(o *Options) { o.Description = "one\ntwo" }, "description"},
		{"description that is too long", func(o *Options) { o.Description = strings.Repeat("x", 121) }, "description"},
		{"empty description", func(o *Options) { o.Description = "" }, "description"},
		{"empty command description", func(o *Options) { o.CommandDescription = "" }, "description"},
		{"scope that is not a scope", func(o *Options) { o.Scopes = []string{"read clients"} }, "scopes"},
		{"scope twice", func(o *Options) { o.Scopes = []string{"read:clients", "read:clients"} }, "scopes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			opts := goodOptions(filepath.Join(parent, "demo"))
			test.change(&opts)

			if opts.Dir != "" {
				opts.Dir = filepath.Join(parent, "demo")
			}

			files, err := Create(opts, miniTree())

			requireInputError(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Nil(t, files)
			assert.Empty(t, entries(t, parent), "nothing is written")
		})
	}
}

func TestCreate_Tokens(t *testing.T) {
	t.Run("replaces the three tokens in the content and nothing else", func(t *testing.T) {
		opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
		opts.Command = "lister"
		opts.License = "Apache-2.0"

		files, err := Create(opts, miniTree())
		require.NoError(t, err)

		assert.Equal(t, []string{".gitignore", "main.go", "plain.txt", "plugin.json", "sub/dir/file"}, files)

		written := readProject(t, opts.Dir)
		assert.Equal(t, "package main // demo lister Apache-2.0\n", written["main.go"])
		assert.Equal(t, "/demo\n", written[".gitignore"])
		assert.Equal(t, "nested\n", written["sub/dir/file"])
		assert.Equal(t, "kept ${{ secrets.TOKEN }} and __lower__ as they are\n", written["plain.txt"], "a file without the suffix is written as it is")
	})

	t.Run("writes the project of the language asked for", func(t *testing.T) {
		opts := goodOptions(filepath.Join(t.TempDir(), "demo"))
		opts.Lang = LangTypeScript

		files, err := Create(opts, miniTree())
		require.NoError(t, err)

		assert.Equal(t, []string{"package.json", "plugin.json"}, files)
		assert.JSONEq(t, `{"name": "demo"}`, readProject(t, opts.Dir)["package.json"])
	})

	tests := []struct {
		name string
		tree fstest.MapFS
		want string
	}{
		{
			name: "a token that is not in the set",
			tree: fstest.MapFS{"go/a.go.tmpl": {Data: []byte("x\n// __DESCRIPTION__\n")}},
			want: "a.go.tmpl has the token __DESCRIPTION__",
		},
		{
			name: "a typo in a token",
			tree: fstest.MapFS{"go/a.go.tmpl": {Data: []byte("// __PLUGIN_NAM__\n")}},
			want: "__PLUGIN_NAM__",
		},
		{
			name: "a token in a file name",
			tree: fstest.MapFS{"go/__PLUGIN_NAME__.go.tmpl": {Data: []byte("x\n")}},
			want: "token __PLUGIN_NAME__ in its name",
		},
		{
			name: "a token in a file without the suffix",
			tree: fstest.MapFS{"go/a.txt": {Data: []byte("__TEAM__\n")}},
			want: "a.txt has the token __TEAM__",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()

			_, err := Create(goodOptions(filepath.Join(parent, "demo")), test.tree)

			requireInputError(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Empty(t, entries(t, parent), "nothing is written")
		})
	}
}

func TestCreate_TemplateNames(t *testing.T) {
	tests := []struct {
		name string
		tree fstest.MapFS
		want string
	}{
		{
			name: "plugin.json",
			tree: fstest.MapFS{"go/plugin.json": {Data: []byte("{}")}},
			want: "plugin.json, which the generated plugin.json already uses",
		},
		{
			name: "plugin.json with the suffix",
			tree: fstest.MapFS{"go/plugin.json.tmpl": {Data: []byte("{}")}},
			want: "plugin.json, which the generated plugin.json already uses",
		},
		{
			name: "a name with and without the suffix",
			tree: fstest.MapFS{"go/a.txt": {Data: []byte("x")}, "go/a.txt.tmpl": {Data: []byte("y")}},
			want: "already uses",
		},
		{
			name: "names that differ only in case",
			tree: fstest.MapFS{"go/README.md": {Data: []byte("x")}, "go/readme.md.tmpl": {Data: []byte("y")}},
			want: "readme.md.tmpl would be written to readme.md, and README.md to README.md, which is the same name when the case is ignored",
		},
		{
			name: "a name that differs from plugin.json only in case",
			tree: fstest.MapFS{"go/Plugin.JSON.tmpl": {Data: []byte("{}")}},
			want: "Plugin.JSON.tmpl would be written to Plugin.JSON, and the generated plugin.json to plugin.json, which is the same name when the case is ignored",
		},
		{
			name: "only the suffix",
			tree: fstest.MapFS{"go/.tmpl": {Data: []byte("x")}},
			want: "has no name",
		},
		{
			name: "no project for the language",
			tree: fstest.MapFS{"ts/a.tmpl": {Data: []byte("x")}},
			want: "no go directory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()

			_, err := Create(goodOptions(filepath.Join(parent, "demo")), test.tree)

			requireInputError(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Empty(t, entries(t, parent))
		})
	}
}

func TestRender_RefusesPathsOutsideTheProject(t *testing.T) {
	for _, path := range []string{"../evil", "a/../../evil", "/etc/passwd", "a/./../../b", ".."} {
		t.Run(path, func(t *testing.T) {
			_, err := goodOptions("x").render([]file{{path: path + templateSuffix, data: []byte("x")}}, []byte("{}"))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not a path inside the project")
		})
	}

	t.Run("a path inside is fine", func(t *testing.T) {
		files, err := goodOptions("x").render([]file{{path: "a/b/c.tmpl", data: []byte("x")}}, []byte("{}"))

		require.NoError(t, err)
		assert.Equal(t, "a/b/c", files[0].path)
	})
}

func TestCreate_Target(t *testing.T) {
	t.Run("a directory that does not exist is created", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "demo")

		_, err := Create(goodOptions(dir), miniTree())
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(dir, "main.go"))
	})

	t.Run("an empty directory is filled and keeps being a directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "demo")
		require.NoError(t, os.Mkdir(dir, 0o700))

		_, err := Create(goodOptions(dir), miniTree())
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(dir, "main.go"))
		assert.Equal(t, []string{"demo"}, entries(t, filepath.Dir(dir)), "no temporary directory is left")
	})

	t.Run("a relative path is made absolute", func(t *testing.T) {
		parent := t.TempDir()
		t.Chdir(parent)

		_, err := Create(goodOptions("demo"), miniTree())
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(parent, "demo", "main.go"))
	})

	t.Run("a directory that is not empty is refused and left as it is", func(t *testing.T) {
		parent := t.TempDir()
		dir := filepath.Join(parent, "demo")
		require.NoError(t, os.Mkdir(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("mine"), 0o644))

		files, err := Create(goodOptions(dir), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), dir)
		assert.Contains(t, err.Error(), "never overwrite a non-empty directory")
		assert.Nil(t, files)
		assert.Equal(t, map[string]string{"mine.txt": "mine"}, readProject(t, dir))
		assert.Equal(t, []string{"demo"}, entries(t, parent), "no temporary directory is left")
	})

	t.Run("a hidden file makes a directory not empty", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "demo")
		require.NoError(t, os.Mkdir(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), nil, 0o644))

		_, err := Create(goodOptions(dir), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), "never overwrite")
	})

	t.Run("a file is refused", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "demo")
		require.NoError(t, os.WriteFile(dir, []byte("mine"), 0o644))

		_, err := Create(goodOptions(dir), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), dir+" exists and is not a directory. Give a directory that does not exist, or an empty one")
		assert.NotContains(t, err.Error(), "symlink")

		data, readErr := os.ReadFile(dir)
		require.NoError(t, readErr)
		assert.Equal(t, "mine", string(data))
	})

	t.Run("a symlink to an empty directory is refused", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		require.NoError(t, os.Mkdir(target, 0o755))

		link := filepath.Join(parent, "demo")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("cannot create a symlink here: %v", err)
		}

		_, err := Create(goodOptions(link), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), link+" is a symlink, which is not followed. Give a directory that does not exist, or an empty one")
		assert.Empty(t, entries(t, target))
	})

	t.Run("a missing parent is refused and not created", func(t *testing.T) {
		parent := t.TempDir()

		_, err := Create(goodOptions(filepath.Join(parent, "missing", "demo")), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), "parent directory does not exist")
		assert.Empty(t, entries(t, parent))
	})

	t.Run("the current directory is refused even when it is empty", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)

		_, err := Create(goodOptions("."), miniTree())

		requireInputError(t, err)
		assert.Contains(t, err.Error(), "is the current directory, which cannot be replaced. Run the command from the parent directory, or give a directory that does not exist")
		assert.Empty(t, entries(t, dir))
	})
}

func TestCreate_FailureLeavesNothing(t *testing.T) {
	failing := func(t *testing.T) {
		t.Helper()

		original := renameDir
		renameDir = func(string, string) error { return errors.New("cannot rename") }

		t.Cleanup(func() { renameDir = original })
	}

	t.Run("a target that did not exist", func(t *testing.T) {
		parent := t.TempDir()
		dir := filepath.Join(parent, "demo")
		failing(t)

		_, err := Create(goodOptions(dir), miniTree())

		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot create "+dir+": cannot rename", "the message names the path")
		assert.NotErrorAs(t, err, new(*InputError), "a failure of the machine is not an error of the input")
		assert.Empty(t, entries(t, parent), "the temporary directory is removed and the target was not made")
	})

	t.Run("a parent directory that cannot be written to", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("the mode of a directory does not stop this user")
		}

		parent := t.TempDir()
		require.NoError(t, os.Chmod(parent, 0o500))
		t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

		dir := filepath.Join(parent, "demo")

		_, err := Create(goodOptions(dir), miniTree())

		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot create "+dir)
		assert.ErrorIs(t, err, fs.ErrPermission)
		assert.NotErrorAs(t, err, new(*InputError), "a failure of the machine is not an error of the input")
	})

	t.Run("an empty target is still there and empty", func(t *testing.T) {
		parent := t.TempDir()
		dir := filepath.Join(parent, "demo")
		require.NoError(t, os.Mkdir(dir, 0o755))
		failing(t)

		_, err := Create(goodOptions(dir), miniTree())

		require.Error(t, err)
		assert.Equal(t, []string{"demo"}, entries(t, parent))
		assert.Empty(t, entries(t, dir))
	})
}

// treeDir writes a template tree to a new directory and returns it as the file
// system that --from would use.
func treeDir(t *testing.T, files map[string]string) fs.FS {
	t.Helper()

	root := t.TempDir()

	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	return os.DirFS(root)
}

func TestCreate_FromDirectory(t *testing.T) {
	t.Run("takes a tree with and without the suffix", func(t *testing.T) {
		opts := goodOptions(filepath.Join(t.TempDir(), "demo"))

		files, err := Create(opts, treeDir(t, map[string]string{
			"go/main.go.tmpl": "// __PLUGIN_NAME__\n",
			"go/plain.go":     "// plain\n",
		}))
		require.NoError(t, err)

		assert.Equal(t, []string{"main.go", "plain.go", "plugin.json"}, files)
		assert.Equal(t, "// demo\n", readProject(t, opts.Dir)["main.go"])
	})

	symlinks := []struct {
		name  string
		build func(t *testing.T, root string)
	}{
		{"a symlinked file", func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "go"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "secret"), []byte("secret"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "secret"), filepath.Join(root, "go", "link.txt")))
		}},
		{"a symlinked directory inside the project", func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "go"), 0o755))
			require.NoError(t, os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "elsewhere", "a.txt"), []byte("x"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "go", "dir")))
		}},
		{"a symlinked project directory", func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "elsewhere", "a.txt"), []byte("x"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "go")))
		}},
		{"a dangling symlink", func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "go"), 0o755))
			require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "go", "link.txt")))
		}},
	}

	for _, test := range symlinks {
		t.Run("refuses "+test.name, func(t *testing.T) {
			root := t.TempDir()
			parent := t.TempDir()

			// The symlink is made first, so a platform that cannot make one skips.
			if err := os.Symlink(root, filepath.Join(parent, "probe")); err != nil {
				t.Skipf("cannot create a symlink here: %v", err)
			}

			test.build(t, root)

			_, err := Create(goodOptions(filepath.Join(parent, "demo")), os.DirFS(root))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "symlink is not followed")
			assert.NoDirExists(t, filepath.Join(parent, "demo"))
			assert.Equal(t, []string{"probe"}, entries(t, parent))
		})
	}
}

func TestCreate_Limits(t *testing.T) {
	file := func(size int) *fstest.MapFile { return &fstest.MapFile{Data: []byte(strings.Repeat("x", size))} }

	// A tree of count small files, or of count empty directories.
	files := func(count int) fstest.MapFS {
		tree := fstest.MapFS{}
		for i := range count {
			tree["go/f"+strings.Repeat("a", i)+".txt"] = file(1)
		}

		return tree
	}

	dirs := func(count int) fstest.MapFS {
		tree := fstest.MapFS{}
		for i := range count {
			tree["go/d"+strings.Repeat("a", i)] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}
		}

		return tree
	}

	tests := []struct {
		name string
		tree fstest.MapFS
		want string // Empty when the tree is accepted.
	}{
		{name: "the most files", tree: files(maxFiles)},
		{name: "one file too many", tree: files(maxFiles + 1), want: "more than 64 files"},
		{name: "the most directories", tree: dirs(maxDirs)},
		{name: "one directory too many", tree: dirs(maxDirs + 1), want: "more than 64 directories"},
		{name: "the biggest file", tree: fstest.MapFS{"go/a.txt": file(maxFileBytes)}},
		{name: "a file one byte too big", tree: fstest.MapFS{"go/a.txt": file(maxFileBytes + 1)}, want: "a.txt is larger than 65536 bytes"},
		{name: "files that fill the total", tree: fstest.MapFS{"go/a": file(maxFileBytes), "go/b": file(maxFileBytes), "go/c": file(maxFileBytes), "go/d": file(maxFileBytes)}},
		{name: "files that are too big together", tree: fstest.MapFS{"go/a": file(maxFileBytes), "go/b": file(maxFileBytes), "go/c": file(maxFileBytes), "go/d": file(maxFileBytes), "go/e": file(1)}, want: "larger than 262144 bytes"},
		{name: "the deepest directory", tree: fstest.MapFS{"go/a/b/c/d/e/f/g/h/file.txt": file(1)}},
		{name: "a directory that goes too deep", tree: fstest.MapFS{"go/a/b/c/d/e/f/g/h/i/file.txt": file(1)}, want: "deeper than 8 levels"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()

			_, err := Create(goodOptions(filepath.Join(parent, "demo")), test.tree)

			if test.want == "" {
				require.NoError(t, err)

				return
			}

			requireInputError(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Empty(t, entries(t, parent), "nothing is written")
		})
	}
}

func TestCheckName(t *testing.T) {
	for _, name := range []string{"demo", "my-plugin", "a1", "x"} {
		assert.NoError(t, CheckName(name), name)
	}

	for _, name := range []string{"", "Demo", "1demo", "-demo", "my_plugin", "my plugin", "demo/x", "../demo", "démo"} {
		assert.ErrorContains(t, CheckName(name), "must start with a lowercase letter", name)
	}
}

func TestEngines(t *testing.T) {
	tests := []struct {
		version string
		want    *plugins.Engines
	}{
		{"1.40.0", &plugins.Engines{Auth0CLI: ">=1.40.0"}},
		{"v1.40.2", &plugins.Engines{Auth0CLI: ">=1.40.2"}},
		{"1.41.0-beta.1", &plugins.Engines{Auth0CLI: ">=1.41.0"}},
		{"1.41.0+build", &plugins.Engines{Auth0CLI: ">=1.41.0"}},
		{"dev", nil},
		{"", nil},
		{"garbage", nil},
		{"1.2", nil},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.want, engines(test.version))
		})
	}
}

func TestPlugin_JSONShape(t *testing.T) {
	opts := goodOptions(filepath.Join(t.TempDir(), "demo"))

	_, err := Create(opts, miniTree())
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(opts.Dir, "plugin.json"))
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))

	permissions := document["commands"].([]any)[0].(map[string]any)["permissions"].(map[string]any)
	assert.Equal(t, []any{"read:clients"}, permissions["scopes"])
	assert.Equal(t, []any{}, permissions["hosts"], "an empty list, not null")
}
