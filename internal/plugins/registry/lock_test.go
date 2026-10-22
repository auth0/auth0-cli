package registry_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// pausingTransport runs pause after a response was received, before the caller
// gets to read it.
type pausingTransport struct {
	inner http.RoundTripper
	pause func(request *http.Request)
}

func (p pausingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := p.inner.RoundTrip(request)
	if err == nil {
		p.pause(request)
	}

	return response, err
}

// cachedVersion returns the version of the index in the cache, after checking
// that the index, its signature and the state belong together.
func cachedVersion(t *testing.T, f *fixture) int {
	t.Helper()

	index, status, err := f.client().Load(t.Context())
	require.NoError(t, err, "the index, the signature and the state of the cache match")
	assert.Equal(t, index.Version, status.Version)
	assert.Equal(t, index.Version, mustHighest(t, f), "the state remembers the version of the cached index")

	return index.Version
}

func TestLock_Refresh(t *testing.T) {
	t.Run("gives up with ErrBusy while another refresh holds the lock, and Load still works", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.client().Refresh(context.Background())
		require.NoError(t, err)

		f.publish(f.builder().SetVersion(2))

		inRequest := make(chan struct{})
		release := make(chan struct{})
		releaseOnce := sync.OnceFunc(func() { close(release) })

		// A failing assertion must not leave the request blocked.
		defer releaseOnce()

		var held atomic.Bool

		f.server.OnRequest(func(path string, _ int) {
			if path == "/index.json" && held.CompareAndSwap(false, true) {
				close(inRequest)
				<-release
			}
		})

		holder := make(chan error, 1)
		go func() {
			_, err := f.client().Refresh(context.Background())
			holder <- err
		}()

		<-inRequest

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = f.client().Refresh(ctx)
		require.ErrorIs(t, err, registry.ErrBusy)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "Another auth0 process is refreshing the registry.")
		assert.Equal(t, 2, f.server.Requests("/index.json"), "the second refresh did not go to the network")

		index, _, err := f.client().Load(t.Context())
		require.NoError(t, err, "Load does not wait for the lock")
		assert.Equal(t, 1, index.Version)

		releaseOnce()
		require.NoError(t, <-holder)
		assert.Equal(t, 2, cachedVersion(t, f))
	})

	t.Run("frees the lock after a refresh fails", func(t *testing.T) {
		f := newFixture(t)
		f.server.Fail("/index.json", http.StatusServiceUnavailable)

		_, err := f.client().Refresh(context.Background())
		require.ErrorIs(t, err, registry.ErrOffline)

		f.server.Fail("/index.json", 0)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = f.client().Refresh(ctx)
		require.NoError(t, err)
	})
}

func TestLock_ConcurrentRefreshes(t *testing.T) {
	t.Run("a refresh waits for the one in progress and does not lose the higher version", func(t *testing.T) {
		f := newFixture(t)
		f.publish(f.builder().SetVersion(3))

		// The first client pauses once it has downloaded version 3, and the registry
		// then moves on to version 5.
		downloaded := make(chan struct{})
		resume := make(chan struct{})
		resumeOnce := sync.OnceFunc(func() { close(resume) })

		// A failing assertion must not leave the download paused.
		defer resumeOnce()

		var paused atomic.Bool

		slow := f.client()
		slow.HTTP = &http.Client{Transport: pausingTransport{
			inner: f.server.Client().Transport,
			pause: func(request *http.Request) {
				if strings.HasSuffix(request.URL.Path, ".sig") && paused.CompareAndSwap(false, true) {
					close(downloaded)
					<-resume
				}
			},
		}}

		results := make(chan error, 2)
		go func() {
			_, err := slow.Refresh(context.Background())
			results <- err
		}()

		<-downloaded
		f.publish(f.builder().SetVersion(5))

		go func() {
			_, err := f.client().Refresh(context.Background())
			results <- err
		}()

		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, 1, f.server.Requests("/index.json"), "the second refresh waits for the lock before it uses the network")

		resumeOnce()
		require.NoError(t, <-results)
		require.NoError(t, <-results)

		assert.Equal(t, 5, cachedVersion(t, f))
	})

	t.Run("many refreshes of rising versions leave the highest one", func(t *testing.T) {
		const refreshes = 8

		f := newFixture(t)

		// The registry publishes the next version for every request of the index.
		// The refreshes take turns because of the lock, so the index and signature
		// that one refresh downloads always belong together.
		f.server.OnRequest(func(path string, count int) {
			if path == "/index.json" {
				f.server.Publish(f.builder().SetVersion(count), f.keyID, f.private)
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		errs := make([]error, refreshes)

		var wg sync.WaitGroup

		for n := range refreshes {
			wg.Add(1)

			go func() {
				defer wg.Done()

				_, errs[n] = f.client().Refresh(ctx)
			}()
		}

		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		assert.Equal(t, refreshes, f.server.Requests("/index.json"))
		assert.Equal(t, refreshes, cachedVersion(t, f))

		var state map[string]any
		require.NoError(t, json.Unmarshal(f.readCache("state.json"), &state))
		assert.EqualValues(t, refreshes, state["highestVersion"])
	})
}

func TestLock_EnsureFreshWhenAnotherProcessIsRefreshing(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	// The helper "shortContext" gives up on the lock after a moment.
	shortContext := func(t *testing.T) context.Context {
		t.Helper()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		t.Cleanup(cancel)

		return ctx
	}

	t.Run("returns the stale cache with a warning", func(t *testing.T) {
		f := newFixture(t)
		client := f.client()
		client.Now = fixedClock(base)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		release, err := registry.HoldLock(f.dir)
		require.NoError(t, err)
		defer release()

		before := f.server.Requests("/index.json")
		client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

		index, status, err := client.EnsureFresh(shortContext(t))

		require.NoError(t, err)
		require.NotNil(t, index)
		assert.True(t, status.Stale)
		assert.True(t, status.Runnable)
		assert.Equal(t, base, status.LastRefresh)
		assert.Contains(t, status.Warning, "Another auth0 process is refreshing the registry")
		assert.Contains(t, status.Warning, "3 days")
		assert.Equal(t, before, f.server.Requests("/index.json"), "nothing was fetched")

		_, err = client.Refresh(shortContext(t))
		assert.ErrorIs(t, err, registry.ErrBusy, "Refresh still reports it")
	})

	t.Run("does not return a cache that expired in the meantime", func(t *testing.T) {
		f := newFixture(t)
		f.publish(f.builder().SetExpires(base.Add(48 * time.Hour)))

		client := f.client()
		client.Now = fixedClock(base)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		release, err := registry.HoldLock(f.dir)
		require.NoError(t, err)
		defer release()

		client.Now = fixedClock(base.Add(49 * time.Hour))

		index, _, err := client.EnsureFresh(shortContext(t))

		assert.Nil(t, index)
		assert.ErrorIs(t, err, registry.ErrExpired)
	})

	t.Run("fails with ErrBusy when nothing is cached", func(t *testing.T) {
		f := newFixture(t)

		release, err := registry.HoldLock(f.dir)
		require.NoError(t, err)
		defer release()

		index, _, err := f.client().EnsureFresh(shortContext(t))

		assert.Nil(t, index)
		assert.ErrorIs(t, err, registry.ErrBusy)
	})

	t.Run("does not fall back when the context is cancelled", func(t *testing.T) {
		f := newFixture(t)
		client := f.client()
		client.Now = fixedClock(base)
		_, err := client.Refresh(context.Background())
		require.NoError(t, err)

		release, err := registry.HoldLock(f.dir)
		require.NoError(t, err)
		defer release()

		client.Now = fixedClock(base.Add(3 * 24 * time.Hour))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		index, _, err := client.EnsureFresh(ctx)

		assert.Nil(t, index)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
