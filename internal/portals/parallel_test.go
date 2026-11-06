package portals

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestForEachParallel(t *testing.T) {
	t.Parallel()

	t.Run("it calls the function once for every index", func(t *testing.T) {
		t.Parallel()

		var calls [20]atomic.Int32

		forEachParallel(len(calls), func(index int) { calls[index].Add(1) })

		for index := range calls {
			assert.Equal(t, int32(1), calls[index].Load(), "index %d", index)
		}
	})

	t.Run("it runs several calls at once, never more than the limit", func(t *testing.T) {
		t.Parallel()

		var (
			running atomic.Int32
			peak    atomic.Int32
			release = make(chan struct{})
			started sync.WaitGroup
		)

		started.Add(maxParallel)

		go func() {
			// The first calls wait here until the limit is reached, which proves they run together. If
			// they never reach it the test fails, and the calls are let go so it does not hang.
			reached := make(chan struct{})

			go func() {
				started.Wait()
				close(reached)
			}()

			select {
			case <-reached:
			case <-time.After(5 * time.Second):
				t.Error("the calls did not run at once")
			}

			close(release)
		}()

		forEachParallel(maxParallel*3, func(int) {
			now := running.Add(1)
			defer running.Add(-1)

			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}

			select {
			case <-release:
			default:
				started.Done()
				<-release
			}
		})

		assert.Equal(t, int32(maxParallel), peak.Load())
	})

	t.Run("it does nothing for no indices", func(t *testing.T) {
		t.Parallel()

		forEachParallel(0, func(int) { t.Error("the function must not be called") })
	})
}
