package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/term"
)

// grace is how long a plugin gets between the request to stop and the kill.
// Tests make it shorter.
var grace = 5 * time.Second

// pollInterval is how often the runner looks whether the process tree is gone
// during the grace period.
const pollInterval = 10 * time.Millisecond

// isTerminal reports whether r is a terminal.
func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)

	return ok && term.IsTerminal(int(file.Fd()))
}

// Spec describes one run of a plugin.
type Spec struct {
	// Bin is the path of the executable. It must contain a path separator, so the
	// plugin is never looked up on PATH.
	Bin  string
	Args []string

	// Env is exactly the environment of the plugin. Nothing is inherited from the
	// CLI, and an empty list means an empty environment. The operating system may
	// still add the few variables it needs to start a process at all.
	Env []string

	// Timeout stops the plugin when it has run this long. Zero means no limit.
	Timeout time.Duration

	// Stdin, Stdout and Stderr are passed to the plugin. A *os.File is handed over
	// as it is, so the plugin gets the descriptor itself, and anything else is
	// copied through a pipe. A nil value is the null device.
	//
	// Unless Interactive is set, stdin that is a terminal is not passed on and the
	// plugin gets the null device instead: a plugin in its own process group would
	// otherwise stop on its first read from the terminal.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Interactive is true for a command that talks to the user. It stays in the
	// process group of the CLI (on Windows, the console group), so the terminal
	// keeps working. On Unix a timeout then stops only the plugin process itself;
	// on Windows the whole tree is still stopped, because it is tracked by a job
	// object.
	Interactive bool

	// The field terminal replaces the check whether Stdin is a terminal, for tests.
	terminal func(io.Reader) bool
}

// ExitError means the plugin ran and exited with a non-zero code. A plugin that
// a signal ended has the code 128 plus the number of the signal.
type ExitError struct {
	Code int
}

// Error returns the message of the error.
func (e *ExitError) Error() string {
	return fmt.Sprintf("the plugin exited with code %d", e.Code)
}

// TimeoutError means the plugin was stopped because it ran longer than its
// timeout.
type TimeoutError struct {
	After time.Duration
}

// Error returns the message of the error.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("the plugin did not finish within %s and was stopped", e.After)
}

// InterruptedError means the plugin was stopped because the context ended, for
// example after Ctrl-C.
type InterruptedError struct{}

// Error returns the message of the error.
func (e *InterruptedError) Error() string {
	return "the plugin was interrupted"
}

// Run starts the plugin and waits for it. It returns nil when the plugin exits
// with code 0, an *ExitError for any other exit, a *TimeoutError when the
// timeout ended it, an *InterruptedError when ctx ended it, and another error
// when the plugin could not be started.
//
// When the timeout or ctx ends the run, the plugin and every process it started
// are asked to stop, and killed if they are still there after the grace period.
//
// On Unix a plugin that is not interactive is in a process group of its own, so
// that covers the whole tree. The kill goes to the group by its number, so a
// process that left the group is not reached, and in the instant between the
// plugin ending and the group being checked the number could in theory be reused.
//
// On Windows the plugin starts suspended and is put in a job object of its own
// before it runs, so everything it starts belongs to the job, and the kill goes
// to the job itself, which no process number can mix up. A process can leave the
// job only if the job allows it, and this one does not. The request to stop is a
// Ctrl-Break sent to the group of a plugin that is not interactive; it is best
// effort, because it needs a console, and it goes by process number. An
// interactive plugin is not asked at all, since Ctrl-C has already reached it
// through the console, and it is killed after the grace period.
//
// When the plugin exits by itself, what it left running is not stopped, on every
// system. If the plugin cannot be put in its job object or resumed, it is killed
// and Run returns an error before the plugin has executed any code.
func Run(ctx context.Context, spec Spec) error {
	if spec.Bin == "" || filepath.Base(spec.Bin) == spec.Bin {
		return fmt.Errorf("the plugin executable %q is not a path", spec.Bin)
	}

	if ctx.Err() != nil {
		return &InterruptedError{}
	}

	cmd := exec.Command(spec.Bin, spec.Args...)
	cmd.Env = spec.Env
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	cmd.WaitDelay = grace

	// A nil Env would make the command inherit the environment of the CLI.
	if cmd.Env == nil {
		cmd.Env = []string{}
	}

	atTerminal := spec.terminal
	if atTerminal == nil {
		atTerminal = isTerminal
	}

	if spec.Stdin != nil && (spec.Interactive || !atTerminal(spec.Stdin)) {
		cmd.Stdin = spec.Stdin
	}

	procs := prepare(cmd, spec.Interactive)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the plugin: %w", err)
	}

	// Release runs on every path from here on. It kills what is left of the tree
	// unless the plugin exited by itself.
	killRest := true

	defer func() { procs.release(killRest) }()

	if err := procs.started(); err != nil {
		// Fail closed: the plugin must not run outside of its tree. Wait is called
		// so the process is reaped and its handle closed.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()

		return fmt.Errorf("could not start the plugin: %w", err)
	}

	// The result of Wait is stored before done is closed, so it is read after done.
	var waitErr error

	done := make(chan struct{})

	go func() {
		waitErr = cmd.Wait()

		close(done)
	}()

	var expired <-chan time.Time

	if spec.Timeout > 0 {
		timer := time.NewTimer(spec.Timeout)
		defer timer.Stop()

		expired = timer.C
	}

	select {
	case <-done:
		killRest = false

		return result(waitErr)
	case <-expired:
		stop(procs, done)

		return &TimeoutError{After: spec.Timeout}
	case <-ctx.Done():
		stop(procs, done)

		return &InterruptedError{}
	}
}

// result turns what Wait returned into the error of Run.
func result(err error) error {
	var exit *exec.ExitError

	switch {
	case err == nil:
		return nil
	case errors.As(err, &exit):
		return &ExitError{Code: exitCode(exit.ProcessState)}
	case errors.Is(err, exec.ErrWaitDelay):
		// Wait reports this only for a plugin that exited with code 0, and something
		// it started kept its output open.
		return nil
	default:
		return err
	}
}

// stop asks the plugin to stop, and kills it when it and what it started are not
// gone when the grace period ends. It returns once the plugin has been waited
// for.
func stop(procs *tree, done <-chan struct{}) {
	procs.terminate()

	deadline := time.NewTimer(grace)
	defer deadline.Stop()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	exited := false

	for {
		if exited && !procs.lingering() {
			return
		}

		select {
		case <-done:
			exited = true
			done = nil
		case <-deadline.C:
			procs.kill()

			if !exited {
				<-done
			}

			return
		case <-ticker.C:
		}
	}
}
