package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/stretchr/testify/assert"
)

func unmarshalSubscribeEvent(t *testing.T, raw string) *managementv2.EventStreamSubscribeEventsResponseContent {
	t.Helper()
	var ev managementv2.EventStreamSubscribeEventsResponseContent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal subscribe event: %v", err)
	}
	return &ev
}

func TestSummarizeEvent_Heartbeat(t *testing.T) {
	ev := unmarshalSubscribeEvent(t, `{"type":"offset-only","offset":"abc123"}`)

	got := summarizeEvent(ev)

	assert.Equal(t, "offset-only", got.eventType)
	assert.True(t, got.isHeartbeat)
	assert.False(t, got.isError)
	assert.Equal(t, "abc123", got.offset)
}

func TestSummarizeEvent_Error(t *testing.T) {
	ev := unmarshalSubscribeEvent(t, `{
		"type":"error",
		"error":{
			"code":"rate_limited",
			"message":"too many requests",
			"offset":"cursor-9"
		}
	}`)

	got := summarizeEvent(ev)

	assert.Equal(t, "error", got.eventType)
	assert.True(t, got.isError)
	assert.False(t, got.isHeartbeat)
	assert.Equal(t, "rate_limited", got.errorCode)
	assert.Equal(t, "too many requests", got.errorMessage)
	assert.Equal(t, "cursor-9", got.offset)
}

func TestSummarizeEvent_ConnectionTimeoutIsRecognized(t *testing.T) {
	// Connection_timeout is filtered upstream; summarizeEvent must still
	// surface it as an error with the exact code so the filter can catch it.
	ev := unmarshalSubscribeEvent(t, `{
		"type":"error",
		"error":{
			"code":"connection_timeout",
			"message":"server is closing the SSE connection"
		}
	}`)

	got := summarizeEvent(ev)

	assert.True(t, got.isError)
	assert.Equal(t, "connection_timeout", got.errorCode)
}

func TestSummarizeEvent_ConcreteEventEnvelope(t *testing.T) {
	ev := unmarshalSubscribeEvent(t, `{
		"type":"user.created",
		"offset":"off-42",
		"event":{
			"specversion":"1.0",
			"type":"user.created",
			"source":"urn:auth0:tenant.example.com",
			"id":"evt_abcdef123456",
			"time":"2026-05-28T10:11:12Z",
			"a0tenant":"tenant",
			"a0stream":"es_xyz",
			"data":{"user":{"user_id":"auth0|1"}}
		}
	}`)

	got := summarizeEvent(ev)

	assert.Equal(t, "user.created", got.eventType)
	assert.False(t, got.isHeartbeat)
	assert.False(t, got.isError)
	assert.Equal(t, "off-42", got.offset)
	assert.Equal(t, "evt_abcdef123456", got.id)
	assert.Equal(t, "urn:auth0:tenant.example.com", got.source)
	assert.Equal(t, 2026, got.time.Year())
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"shorter than width pads with spaces", "abc", 5, "abc  "},
		{"equal to width returned unchanged", "abcde", 5, "abcde"},
		{"longer than width returned unchanged", "abcdef", 5, "abcdef"},
		{"empty string fills full width", "", 4, "    "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, padRight(tc.in, tc.width))
		})
	}
}

func TestShortOffset(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"short offset returned unchanged", "abc123", "abc123"},
		{"exactly 12 chars returned unchanged", "abcdef123456", "abcdef123456"},
		{"long offset truncated", "abcdef123456ghijkl", "abcdef12…ijkl"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shortOffset(tc.in))
		})
	}
}

func TestInvalidEventTypesError_Single(t *testing.T) {
	err := invalidEventTypesError([]string{"new.users"})
	require := assert.New(t)
	require.Error(err)
	msg := err.Error()
	require.Contains(msg, `"new.users"`)
	require.Contains(msg, "--list-event-types")
	// SDK's internal type name must not leak into user output.
	require.NotContains(msg, "EventStreamSubscribeEventsEventTypeEnum")
}

func TestInvalidEventTypesError_Multiple(t *testing.T) {
	err := invalidEventTypesError([]string{"new.users", "free.users", "totally.bogus"})
	require := assert.New(t)
	require.Error(err)
	msg := err.Error()
	// All three bad values must be surfaced.
	require.Contains(msg, `"new.users"`)
	require.Contains(msg, `"free.users"`)
	require.Contains(msg, `"totally.bogus"`)
	require.Contains(msg, "invalid --event-type values")
	require.Contains(msg, "--list-event-types")
	require.NotContains(msg, "EventStreamSubscribeEventsEventTypeEnum")
}

func TestReconnectBackoff(t *testing.T) {
	// Attempt 0 with no server directive (a healthy connection rotation)
	// reconnects immediately.
	assert.Equal(t, time.Duration(0), reconnectBackoff(0, 0))
	assert.Equal(t, time.Duration(0), reconnectBackoff(-3, 0))

	// With full jitter the delay is always in [0, ceiling) and the ceiling
	// grows exponentially but never exceeds reconnectMaxBackoff.
	for attempt := 1; attempt <= 20; attempt++ {
		ceiling := reconnectBaseBackoff << (attempt - 1)
		if ceiling > reconnectMaxBackoff || ceiling <= 0 {
			ceiling = reconnectMaxBackoff
		}
		for range 100 {
			d := reconnectBackoff(attempt, 0)
			assert.GreaterOrEqual(t, d, time.Duration(0), "attempt %d delay must be non-negative", attempt)
			assert.Less(t, d, ceiling, "attempt %d delay must be below its ceiling", attempt)
			assert.LessOrEqual(t, d, reconnectMaxBackoff, "attempt %d delay must never exceed the max", attempt)
		}
	}

	// A server-advertised retry acts as a floor: even on a healthy rotation
	// (attempt 0) we never reconnect faster than the server asked.
	assert.Equal(t, 3*time.Second, reconnectBackoff(0, 3*time.Second))

	// When the server retry exceeds the jitter ceiling, it always wins.
	for range 100 {
		assert.GreaterOrEqual(t, reconnectBackoff(2, time.Minute), time.Minute,
			"server retry must act as a floor regardless of jitter")
	}
}

func TestColorForEventType(t *testing.T) {
	// We don't assert exact escape codes (those live in the ansi package);
	// we just confirm the original event type is preserved inside the colored
	// output so users still see the right text.
	cases := []string{
		"user.created",
		"user.updated",
		"user.deleted",
		"organization.member.added",
		"organization.member.removed",
		"role.assigned",
		"unknown.event",
		"error",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			out := colorForEventType(c)
			assert.True(t, strings.Contains(out, c), "colored output %q must contain %q", out, c)
		})
	}
}
