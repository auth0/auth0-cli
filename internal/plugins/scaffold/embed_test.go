package scaffold

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// embeddedBudgetBytes is the most the embedded templates may add to the binary.
// The templates were 100,431 bytes when it was set, plus about a quarter. Raise
// it on purpose, with a reason, and never to make a test pass.
const embeddedBudgetBytes = 125_000

// A template tree never holds lock files, which belong to the project that is
// created and not to the template, or the directories of installed dependencies.
var (
	forbiddenFiles = regexp.MustCompile(`^(go\.sum|package-lock\.json|bun\.lockb?|yarn\.lock|pnpm-lock\.yaml)(\.tmpl)?$`)
	forbiddenDirs  = map[string]bool{"node_modules": true, "vendor": true}
)

// treeProblems returns everything wrong with a template tree: more bytes than the
// budget, a file that looks binary, a file that is a lock file, a directory of
// dependencies, or a file without the template suffix.
func treeProblems(templates fs.FS, budget int) []string {
	var (
		problems []string
		total    int
	)

	err := fs.WalkDir(templates, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if forbiddenDirs[entry.Name()] {
				problems = append(problems, name+" is a directory of dependencies")
			}

			return nil
		}

		if forbiddenFiles.MatchString(entry.Name()) {
			problems = append(problems, name+" is a lock file")
		}

		if path.Ext(name) != templateSuffix {
			problems = append(problems, name+" has no "+templateSuffix+" suffix")
		}

		data, err := fs.ReadFile(templates, name)
		if err != nil {
			return err
		}

		if bytes.IndexByte(data, 0) >= 0 {
			problems = append(problems, name+" holds a NUL byte, so it looks binary")
		}

		total += len(data)

		return nil
	})
	if err != nil {
		problems = append(problems, err.Error())
	}

	if total > budget {
		problems = append(problems, "the templates are larger than the budget")
	}

	return problems
}

func TestEmbedded(t *testing.T) {
	templates := Embedded()

	assert.Empty(t, treeProblems(templates, embeddedBudgetBytes))

	t.Run("holds a project for each language", func(t *testing.T) {
		for _, lang := range []Lang{LangGo, LangTypeScript} {
			info, err := fs.Stat(templates, string(lang))
			require.NoError(t, err)
			assert.True(t, info.IsDir(), lang)
		}

		top, err := fs.ReadDir(templates, ".")
		require.NoError(t, err)
		assert.Len(t, top, 2, "only go and ts")
	})

	t.Run("keeps the files whose names start with a dot", func(t *testing.T) {
		for _, name := range []string{"go/.gitignore.tmpl", "ts/.gitignore.tmpl", "go/.github/workflows/ci.yml.tmpl"} {
			_, err := fs.Stat(templates, name)
			assert.NoError(t, err, name)
		}
	})

	t.Run("holds go.mod only as a template", func(t *testing.T) {
		_, err := fs.Stat(templates, "go/go.mod.tmpl")
		require.NoError(t, err)

		_, err = fs.Stat(templates, "go/go.mod")
		assert.Error(t, err)
	})

	t.Run("is within the limits that --from has", func(t *testing.T) {
		for _, lang := range []Lang{LangGo, LangTypeScript} {
			tree, err := readTree(templates, lang)
			require.NoError(t, err, lang)
			assert.LessOrEqual(t, len(tree), maxFiles)
		}
	})
}

func TestTreeProblems(t *testing.T) {
	file := func(data string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(data)} }

	tests := []struct {
		name string
		tree fstest.MapFS
		want []string
	}{
		{name: "a good tree", tree: fstest.MapFS{"go/main.go.tmpl": file("package main\n")}},
		{name: "over the budget", tree: fstest.MapFS{"go/a.tmpl": file(strings.Repeat("x", 600)), "ts/b.tmpl": file(strings.Repeat("y", 600))}, want: []string{"larger than the budget"}},
		{name: "exactly the budget", tree: fstest.MapFS{"go/a.tmpl": file(strings.Repeat("x", 1000))}},
		{name: "a binary file", tree: fstest.MapFS{"go/logo.png.tmpl": file("\x89PNG\x00\x01")}, want: []string{"logo.png.tmpl holds a NUL byte"}},
		{name: "go.sum", tree: fstest.MapFS{"go/go.sum.tmpl": file("x")}, want: []string{"go/go.sum.tmpl is a lock file"}},
		{name: "go.sum without the suffix", tree: fstest.MapFS{"go/go.sum": file("x")}, want: []string{"go/go.sum is a lock file", "go/go.sum has no .tmpl suffix"}},
		{name: "package-lock.json", tree: fstest.MapFS{"ts/package-lock.json.tmpl": file("{}")}, want: []string{"is a lock file"}},
		{name: "bun.lock", tree: fstest.MapFS{"ts/bun.lock.tmpl": file("x")}, want: []string{"is a lock file"}},
		{name: "bun.lockb", tree: fstest.MapFS{"ts/bun.lockb.tmpl": file("x")}, want: []string{"is a lock file"}},
		{name: "yarn.lock", tree: fstest.MapFS{"ts/yarn.lock.tmpl": file("x")}, want: []string{"is a lock file"}},
		{name: "pnpm-lock.yaml", tree: fstest.MapFS{"ts/pnpm-lock.yaml.tmpl": file("x")}, want: []string{"is a lock file"}},
		{name: "node_modules", tree: fstest.MapFS{"ts/node_modules/x/index.js.tmpl": file("x")}, want: []string{"ts/node_modules is a directory of dependencies"}},
		{name: "vendor", tree: fstest.MapFS{"go/vendor/x/x.go.tmpl": file("x")}, want: []string{"go/vendor is a directory of dependencies"}},
		{name: "a file named like a lock file in a longer name", tree: fstest.MapFS{"go/my-go.sum.tmpl": file("x")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			problems := treeProblems(test.tree, 1000)

			require.Len(t, problems, len(test.want), problems)

			for i, want := range test.want {
				assert.Contains(t, problems[i], want)
			}
		})
	}
}

// networkFailure matches what the go and bun commands say when they cannot
// download.
var networkFailure = regexp.MustCompile(`dial tcp|no such host|i/o timeout|proxy.golang.org|connection refused|network is unreachable|TLS handshake|ConnectionRefused|FailedToOpenSocket|ConnectionClosed|ENOTFOUND|Unable to connect`)

// TestEmbedded_BuildsAndPasses scaffolds each project and builds, vets and tests
// it with the tools of its language, the way a person would. It needs the tool
// and the package registry (or a warm cache), so a case is skipped when either is
// missing.
func TestEmbedded_BuildsAndPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("builds projects with the go and bun commands")
	}

	tests := []struct {
		name    string
		lang    Lang
		plugin  string
		command string
	}{
		{"go", LangGo, "demo", "demo"},
		{"go with a command that is not the root command", LangGo, "demo", "list"},
		{"go named like a standard library package", LangGo, "net", "net"},
		{"ts", LangTypeScript, "demo", "demo"},
		{"ts with a command that is not the root command", LangTypeScript, "demo", "list"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tool, steps := "go", [][]string{{"mod", "tidy"}, {"vet", "./..."}, {"test", "./..."}, {"build", "-o", test.plugin, "."}}
			if test.lang == LangTypeScript {
				tool, steps = "bun", [][]string{{"install"}, {"run", "typecheck"}, {"test"}}
			}

			bin, err := exec.LookPath(tool)
			if err != nil {
				t.Skipf("the %s command is not on the PATH", tool)
			}

			opts := goodOptions(filepath.Join(t.TempDir(), test.plugin))
			opts.Name, opts.ID, opts.Command, opts.Lang = test.plugin, "auth0/"+test.plugin, test.command, test.lang

			_, err = Create(opts, Embedded())
			require.NoError(t, err)

			for i, args := range steps {
				cmd := exec.Command(bin, args...)
				cmd.Dir = opts.Dir
				// The project is its own module, whatever the environment of the test says.
				cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "GO111MODULE=on")

				output, err := cmd.CombinedOutput()
				if err != nil && i == 0 && networkFailure.Match(output) {
					t.Skipf("%s %s needs the network: %s", tool, args[0], output)
				}

				require.NoError(t, err, "%s %s:\n%s", tool, strings.Join(args, " "), output)
			}

			if test.lang == LangGo {
				assert.FileExists(t, filepath.Join(opts.Dir, "go.sum"), "the project writes it, the template does not")
			}
		})
	}
}
