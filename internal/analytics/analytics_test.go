package analytics

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateEventName(t *testing.T) {
	t.Run("generates from root command run", func(t *testing.T) {
		want := "CLI - Auth0 - Action"
		got := generateEventName("auth0", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from top-level command run", func(t *testing.T) {
		want := "CLI - Auth0 - Apps - Action"
		got := generateEventName("auth0 apps", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from subcommand run", func(t *testing.T) {
		want := "CLI - Apps - List - Action"
		got := generateEventName("auth0 apps list", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from deep subcommand run", func(t *testing.T) {
		want := "CLI - Apis - Scopes List - Action"
		got := generateEventName("auth0 apis scopes list", "Action")
		assert.Equal(t, want, got)
	})
}

func TestGenerateRunEventName(t *testing.T) {
	t.Run("generates from root command run", func(t *testing.T) {
		want := "CLI - Auth0 - Run"
		got := generateRunEventName("auth0")
		assert.Equal(t, want, got)
	})

	t.Run("generates from top-level command run", func(t *testing.T) {
		want := "CLI - Auth0 - Apps - Run"
		got := generateRunEventName("auth0 apps")
		assert.Equal(t, want, got)
	})

	t.Run("generates from subcommand run", func(t *testing.T) {
		want := "CLI - Apps - List - Run"
		got := generateRunEventName("auth0 apps list")
		assert.Equal(t, want, got)
	})

	t.Run("generates from deep subcommand run", func(t *testing.T) {
		want := "CLI - Apis - Scopes List - Run"
		got := generateRunEventName("auth0 apis scopes list")
		assert.Equal(t, want, got)
	})
}

func TestNewEvent(t *testing.T) {
	t.Run("creates a new event instance", func(t *testing.T) {
		event := newEvent("event", "id")
		// Assert that the interval between the event timestamp and now is within 1 second
		assert.WithinDuration(t, time.Now(), time.Unix(0, event.Timestamp * int64(1000000)), 1 * time.Second)
		assert.Equal(t, event.App, appID)
		assert.Equal(t, event.Event, "event")
		assert.Equal(t, event.ID, "id")
		assert.Equal(t, event.Properties[osKey], runtime.GOOS)
		assert.Equal(t, event.Properties[archKey], runtime.GOARCH)
	})
}
