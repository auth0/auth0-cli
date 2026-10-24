package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helperSentinel as the first argument also makes the test binary a helper
// process, for runs with an empty environment.
const helperSentinel = "--helper"

// helperModes are what the test binary can do when it runs as a plugin. Other
// test files add the ones that only work on some systems.
var helperModes = map[string]func(args []string){
	"exit": func(args []string) {
		code, _ := strconv.Atoi(args[0])
		exitHelper(code)
	},
	"echo": func([]string) {
		fmt.Fprintln(os.Stdout, "to stdout")
		fmt.Fprintln(os.Stderr, "to stderr")
	},
	"env": func([]string) {
		for _, entry := range os.Environ() {
			fmt.Fprintln(os.Stdout, entry)
		}
	},
	"stdin": func([]string) {
		data, _ := io.ReadAll(os.Stdin)
		fmt.Fprintf(os.Stdout, "stdin=%d", len(data))
	},
	"sleep": func([]string) {
		time.Sleep(time.Minute)
	},
	"pause": func([]string) {
		time.Sleep(time.Second)
	},

	// Beat <dir> [ignore-term] writes the first beat to <dir>/beat and its process
	// number to <dir>/ready, then rewrites <dir>/beat every few milliseconds with
	// a counter. The first beat comes before ready, so a test that stops the
	// process as soon as it is ready always finds a beat. It ends by itself after a
	// while, in case a test leaves it behind.
	"beat": func(args []string) {
		if len(args) > 1 && args[1] == "ignore-term" {
			ignoreStop()
		}

		_ = os.WriteFile(filepath.Join(args[0], "beat"), []byte("0"), 0o600)
		_ = os.WriteFile(filepath.Join(args[0], "ready"), []byte(strconv.Itoa(os.Getpid())), 0o600)

		for i := 0; i < 1000; i++ {
			if err := os.WriteFile(filepath.Join(args[0], "beat"), []byte(strconv.Itoa(i)), 0o600); err != nil {
				return
			}

			time.Sleep(20 * time.Millisecond)
		}
	},

	// Spawn <dir> [ignore-term] starts a grandchild that beats, and waits to be
	// stopped itself.
	"spawn": func(args []string) {
		child := helperChild(append([]string{"beat"}, args...)...)

		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitHelper(3)
		}

		time.Sleep(time.Minute)
	},

	// Daemon exits at once and leaves a process behind that keeps its output
	// open.
	"daemon": func([]string) {
		child := helperChild("pause")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		if err := child.Start(); err != nil {
			exitHelper(3)
		}
	},
}

// helperChild returns a command that runs a mode of the test binary with a
// minimal environment, for a helper that starts a process of its own.
func helperChild(args ...string) *exec.Cmd {
	child := exec.Command(executable(), args...)
	child.Env = []string{"GO_WANT_HELPER=1"}

	return child
}

func executable() string {
	path, err := os.Executable()
	if err != nil {
		panic(err)
	}

	return path
}

func TestMain(m *testing.M) {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == helperSentinel {
		args = args[1:]
	} else if os.Getenv("GO_WANT_HELPER") != "1" {
		grace = 300 * time.Millisecond

		os.Exit(m.Run())
	}

	if mode, ok := helperModes[args[0]]; ok {
		mode(args[1:])
	}

	exitHelper(0)
}

// helper returns a spec that runs a mode of the test binary with a minimal
// environment.
func helper(args ...string) Spec {
	return Spec{Bin: executable(), Args: args, Env: []string{"GO_WANT_HELPER=1"}}
}

// waitFor waits until the file exists.
func waitFor(t *testing.T, path string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_, err := os.Stat(path)

		return err == nil
	}, 30*time.Second, 5*time.Millisecond, "%s never appeared", path)
}

// beats returns what the process that beats last wrote.
func beats(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "beat"))
	require.NoError(t, err)

	return string(data)
}

// requireStopped requires that the process that beats in dir has stopped
// beating, and was beating before.
func requireStopped(t *testing.T, dir string) {
	t.Helper()

	before := beats(t, dir)

	time.Sleep(300 * time.Millisecond)

	assert.Equal(t, before, beats(t, dir), "the process is still running")
}

// requireStoppedByRun fails when Run took about as long as the helper processes
// live. Without it a tree that was never stopped passes requireStopped, because
// Run only returns when the helpers have ended on their own.
func requireStoppedByRun(t *testing.T, started time.Time) {
	t.Helper()

	require.Less(t, time.Since(started), 30*time.Second, "Run did not stop the process tree, it waited for the processes to end")
}

func TestRun_Results(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"exit 0 is success", []string{"exit", "0"}, 0},
		{"exit 7 is an exit error", []string{"exit", "7"}, 7},
		{"exit 1 is an exit error", []string{"exit", "1"}, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := Run(t.Context(), helper(test.args...))

			if test.want == 0 {
				require.NoError(t, err)

				return
			}

			var exit *ExitError
			require.ErrorAs(t, err, &exit)
			assert.Equal(t, test.want, exit.Code)
		})
	}
}

func TestRun_PassesOutputThrough(t *testing.T) {
	t.Parallel()

	stdout, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	require.NoError(t, err)

	defer stdout.Close()

	var stderr bytes.Buffer

	spec := helper("echo")
	spec.Stdout = stdout
	spec.Stderr = &stderr

	require.NoError(t, Run(t.Context(), spec))

	written, err := os.ReadFile(stdout.Name())
	require.NoError(t, err)

	assert.Equal(t, "to stdout", strings.TrimSpace(string(written)))
	assert.Equal(t, "to stderr", strings.TrimSpace(stderr.String()))
}

func TestRun_UsesOnlyTheEnvironmentOfTheSpec(t *testing.T) {
	t.Setenv("AUTH0_CLIENT_SECRET", "leaked")
	t.Setenv("AWS_PROFILE", "leaked")

	tests := []struct {
		name string
		spec Spec
		want []string
	}{
		{"a list", Spec{Bin: executable(), Args: []string{"env"}, Env: []string{"GO_WANT_HELPER=1", "KEPT=yes"}}, []string{"GO_WANT_HELPER=1", "KEPT=yes"}},
		{"an empty list", Spec{Bin: executable(), Args: []string{helperSentinel, "env"}, Env: []string{}}, nil},
		{"no list", Spec{Bin: executable(), Args: []string{helperSentinel, "env"}}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer

			spec := test.spec
			spec.Stdout = &stdout

			require.NoError(t, Run(t.Context(), spec))

			got := strings.Fields(stdout.String())

			assert.NotContains(t, stdout.String(), "leaked")

			for _, entry := range test.want {
				assert.Contains(t, got, entry)
			}

			// The system may add variables it needs to start a process, but none
			// of the CLI.
			assert.LessOrEqual(t, len(got), len(test.want)+4, "unexpected environment: %v", got)
		})
	}
}

func TestRun_Stdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		terminal    bool
		interactive bool
		want        string
	}{
		{"is passed on when it is not a terminal", false, false, "stdin=5"},
		{"is closed for a command that is not interactive when it is a terminal", true, false, "stdin=0"},
		{"is passed on to an interactive command when it is a terminal", true, true, "stdin=5"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer

			spec := helper("stdin")
			spec.Stdin = strings.NewReader("hello")
			spec.Stdout = &stdout
			spec.Interactive = test.interactive
			spec.terminal = func(io.Reader) bool { return test.terminal }

			require.NoError(t, Run(t.Context(), spec))
			assert.Equal(t, test.want, stdout.String())
		})
	}

	t.Run("is the null device when there is none", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer

		spec := helper("stdin")
		spec.Stdout = &stdout

		require.NoError(t, Run(t.Context(), spec))
		assert.Equal(t, "stdin=0", stdout.String())
	})
}

func TestIsTerminal(t *testing.T) {
	t.Parallel()

	assert.False(t, isTerminal(strings.NewReader("")), "a reader is not a terminal")

	file, err := os.Create(filepath.Join(t.TempDir(), "file"))
	require.NoError(t, err)

	defer file.Close()

	assert.False(t, isTerminal(file), "a file is not a terminal")
}

func TestRun_DoesNotStart(t *testing.T) {
	t.Parallel()

	t.Run("a missing executable", func(t *testing.T) {
		t.Parallel()

		spec := helper("exit", "0")
		spec.Bin = filepath.Join(t.TempDir(), "missing")

		err := Run(t.Context(), spec)

		// Windows reports a missing file as a failed lookup of the program.
		require.True(t, errors.Is(err, fs.ErrNotExist) || errors.Is(err, exec.ErrNotFound), "got %v", err)

		var exit *ExitError
		assert.False(t, errors.As(err, &exit))
	})

	t.Run("a name that would be looked up on PATH", func(t *testing.T) {
		t.Parallel()

		spec := helper("exit", "0")
		spec.Bin = filepath.Base(spec.Bin)

		assert.ErrorContains(t, Run(t.Context(), spec), "is not a path")
	})

	t.Run("an empty name", func(t *testing.T) {
		t.Parallel()

		assert.ErrorContains(t, Run(t.Context(), Spec{}), "is not a path")
	})

	t.Run("a context that has ended", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		// The executable does not exist: the context is looked at first.
		spec := helper("exit", "0")
		spec.Bin = filepath.Join(t.TempDir(), "missing")

		var interrupted *InterruptedError
		assert.ErrorAs(t, Run(ctx, spec), &interrupted)
	})
}

func TestRun_Timeout(t *testing.T) {
	t.Parallel()

	spec := helper("sleep")
	spec.Timeout = 50 * time.Millisecond

	err := Run(t.Context(), spec)

	var timeout *TimeoutError
	require.ErrorAs(t, err, &timeout)
	assert.Equal(t, 50*time.Millisecond, timeout.After)
}

func TestRun_TimeoutStopsTheWholeTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// The limit is long because a process can take a second to start when the
	// tests run with the race detector.
	spec := helper("spawn", dir)
	spec.Timeout = 3 * time.Second

	started := time.Now()

	var timeout *TimeoutError
	require.ErrorAs(t, Run(t.Context(), spec), &timeout)
	require.FileExists(t, filepath.Join(dir, "ready"), "the grandchild did not start before the timeout")
	requireStoppedByRun(t, started)

	requireStopped(t, dir)
}

func TestRun_CancelStopsTheWholeTree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		waitFor(t, filepath.Join(dir, "ready"))
		cancel()
	}()

	started := time.Now()

	var interrupted *InterruptedError
	require.ErrorAs(t, Run(ctx, helper("spawn", dir)), &interrupted)
	requireStoppedByRun(t, started)

	requireStopped(t, dir)
}

func TestRun_KillsWhatIgnoresTheRequestToStop(t *testing.T) {
	t.Parallel()

	// Both start a process that beats and ignores the request to stop. The first
	// is the plugin itself, the second one it started.
	for name, args := range map[string][]string{
		"the plugin":                   {"beat"},
		"a process the plugin started": {"spawn"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var cancelled time.Time

			go func() {
				waitFor(t, filepath.Join(dir, "ready"))

				cancelled = time.Now()

				cancel()
			}()

			var interrupted *InterruptedError
			require.ErrorAs(t, Run(ctx, helper(append(args, dir, "ignore-term")...)), &interrupted)

			assert.GreaterOrEqual(t, time.Since(cancelled), grace, "it was killed before the grace period ended")
			requireStopped(t, dir)
		})
	}
}

func TestRun_SucceedsWhenAProcessKeepsTheOutputOpen(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	spec := helper("daemon")
	spec.Stdout = &stdout

	assert.NoError(t, Run(t.Context(), spec))
}
