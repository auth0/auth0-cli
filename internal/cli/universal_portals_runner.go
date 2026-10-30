package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// The oldest Node.js the tool runs on.
const (
	universalPortalsNodeMajor = 20
	universalPortalsNodeMinor = 12
)

var nodeVersionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)`)

// universalPortalsRun describes one run of the embedded tool.
type universalPortalsRun struct {
	// Node is the path of the node executable.
	Node string
	// Script is the text of the tool.
	Script []byte
	// Args follow the script path: the command (setup or audit) and its flags.
	Args []string
	// Env is added to the environment, as KEY=value.
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// runUniversalPortalsScript writes the tool to a private temporary file, runs it with Node.js
// and returns its exit code. The tool talks to the Auth0 Management API by calling this CLI
// again, so the login and the token stay inside the CLI.
func runUniversalPortalsScript(run universalPortalsRun) (int, error) {
	// The folder is private to the current user, and removed when the run ends.
	dir, err := os.MkdirTemp("", "auth0-universal-portals-")
	if err != nil {
		return 0, fmt.Errorf("failed to create a temporary folder for the tool: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "universal-portals.mjs")
	if err := os.WriteFile(path, run.Script, 0o600); err != nil {
		return 0, fmt.Errorf("failed to write the tool to a temporary file: %w", err)
	}

	cmd := exec.Command(run.Node, append([]string{path}, run.Args...)...)
	cmd.Env = universalPortalsEnv(os.Environ(), run.Env)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = run.Stdin, run.Stdout, run.Stderr

	// The root command exits as soon as it gets Ctrl+C, which would cut off the tool's last message.
	// The terminal sends Ctrl+C to the tool too, so this process only has to wait for it.
	signal.Reset(os.Interrupt, syscall.SIGTERM)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start Node.js: %w", err)
	}

	finished := make(chan struct{})
	go func() {
		for {
			select {
			case received := <-signals:
				// Only a SIGTERM sent to this process alone needs passing on.
				if received == syscall.SIGTERM {
					_ = cmd.Process.Signal(received)
				}
			case <-finished:
				return
			}
		}
	}()

	err = cmd.Wait()
	close(finished)

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// A tool ended by a signal has no exit code, so use the one a shell reports: 128 plus the
		// signal number, which is 130 for Ctrl+C.
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("node stopped unexpectedly: %w", err)
	}

	return 0, nil
}

// universalPortalsEnv adds extra to base. A proxy left over in the environment is dropped, so it
// cannot switch the tool to another way of calling the API.
func universalPortalsEnv(base, extra []string) []string {
	env := make([]string, 0, len(base)+len(extra))
	for _, entry := range base {
		if strings.HasPrefix(entry, "AUTH0_CLI_PROXY_URL=") || strings.HasPrefix(entry, "AUTH0_CLI_PROXY_TOKEN=") {
			continue
		}
		env = append(env, entry)
	}

	// When a name appears twice, the last value is used.
	return append(env, extra...)
}

// universalPortalsTenantFlag returns the value of --tenant in the arguments, or "". The flags
// belong to the tool, so the CLI only reads this one to tell the tool which tenant to use.
func universalPortalsTenantFlag(args []string) string {
	tenant := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--tenant" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			tenant = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--tenant="):
			tenant = strings.TrimPrefix(args[i], "--tenant=")
		}
	}

	return tenant
}

// checkUniversalPortalsNodeVersion fails if node is older than the tool supports.
func checkUniversalPortalsNodeVersion(node string) error {
	output, err := exec.Command(node, "--version").Output()
	if err != nil {
		return fmt.Errorf("could not read the Node.js version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	match := nodeVersionPattern.FindStringSubmatch(version)
	if match == nil {
		return fmt.Errorf("could not read the Node.js version from %q", version)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	if major < universalPortalsNodeMajor || (major == universalPortalsNodeMajor && minor < universalPortalsNodeMinor) {
		return fmt.Errorf(
			"node %s is too old: this command needs Node.js %d.%d or newer",
			version, universalPortalsNodeMajor, universalPortalsNodeMinor,
		)
	}

	return nil
}
