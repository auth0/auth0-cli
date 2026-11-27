package cli

import (
	"context"
	"sync"
)

// interruptGuard lets a command that started other processes stop them before
// the CLI exits on Ctrl-C. Without a holder the interrupt ends the process at
// once, as it always did. While there is one, the interrupt only cancels the
// context, the holder cleans up and returns, and Execute then exits with the code
// of an interrupt.
type interruptGuard struct {
	mu          sync.Mutex
	holders     int
	interrupted bool
}

type interruptGuardKey struct{}

// withInterruptGuard returns a context that carries the guard.
func withInterruptGuard(ctx context.Context, guard *interruptGuard) context.Context {
	return context.WithValue(ctx, interruptGuardKey{}, guard)
}

// holdInterrupt makes an interrupt wait for the caller, until the function it
// returns is called. A context without a guard, as in tests, has nothing to hold.
func holdInterrupt(ctx context.Context) (release func()) {
	guard, ok := ctx.Value(interruptGuardKey{}).(*interruptGuard)
	if !ok {
		return func() {}
	}

	guard.mu.Lock()
	guard.holders++
	guard.mu.Unlock()

	var once sync.Once

	return func() {
		once.Do(func() {
			guard.mu.Lock()
			guard.holders--
			guard.mu.Unlock()
		})
	}
}

// interrupt reports whether a holder will deal with the interrupt. When it
// returns false, nothing is running that needs to be stopped and the caller ends
// the process.
func (g *interruptGuard) interrupt() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.holders == 0 {
		return false
	}

	g.interrupted = true

	return true
}

// wasInterrupted reports whether an interrupt arrived while a holder ran.
func (g *interruptGuard) wasInterrupted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.interrupted
}

// interrupted reports whether an interrupt arrived while a command held the
// guard of the context.
func interrupted(ctx context.Context) bool {
	guard, ok := ctx.Value(interruptGuardKey{}).(*interruptGuard)

	return ok && guard.wasInterrupted()
}
