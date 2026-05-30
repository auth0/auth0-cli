package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	managementoption "github.com/auth0/go-auth0/v2/management/option"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	eventSubscribeFrom = Flag{
		Name:     "From",
		LongForm: "from",
		Help: "Opaque cursor token representing the position in the stream. " +
			"If not provided, the stream starts from the latest events. Use the " +
			"`offset` printed when the connection ends to resume from where you left off.",
	}

	eventSubscribeFromTimestamp = Flag{
		Name:     "From Timestamp",
		LongForm: "from-timestamp",
		Help: "RFC-3339 timestamp indicating where to start streaming events from. " +
			"Use this on the initial query when no cursor (--from) is available; " +
			"prefer --from on subsequent runs as it is more accurate.",
	}

	eventSubscribeEventType = Flag{
		Name:     "Event Type",
		LongForm: "event-type",
		Help: "Event type(s) to listen for. Specify multiple times for multiple types " +
			"(e.g. --event-type user.created --event-type user.updated). " +
			"If not provided, all event types are streamed.",
	}

	eventSubscribeVerbose = Flag{
		Name:      "Verbose",
		LongForm:  "verbose",
		ShortForm: "v",
		Help:      "Print the full JSON payload after each event summary line.",
	}

	eventSubscribeShowHeartbeats = Flag{
		Name:     "Show Heartbeats",
		LongForm: "show-heartbeats",
		Help: "Show every `offset-only` heartbeat as its own line. " +
			"By default heartbeats are silently tracked and only the latest cursor " +
			"is reported on disconnect.",
	}

	eventSubscribeOutputFile = Flag{
		Name:     "Output File",
		LongForm: "output-file",
		Help: "Append every received event as a JSON line to this file (raw payload). " +
			"Independent of the stdout format.",
	}

	eventSubscribeNoReconnect = Flag{
		Name:     "No Reconnect",
		LongForm: "no-reconnect",
		Help: "Disable automatic reconnection. By default the stream resumes from " +
			"the last cursor after the server drops the connection, so no events " +
			"are missed. With this flag the command exits when the stream ends.",
	}

	eventSubscribeMaxReconnects = Flag{
		Name:     "Max Reconnects",
		LongForm: "max-reconnects",
		Help: "Maximum number of consecutive failed reconnect attempts before " +
			"giving up. 0 (default) keeps retrying as long as the stream is " +
			"making progress. Ignored when --no-reconnect is set.",
	}

	eventSubscribeListEventTypes = Flag{
		Name:     "List Event Types",
		LongForm: "list-event-types",
		Help: "Print every event type accepted by --event-type and exit, " +
			"without opening a subscription.",
	}
)

// supportedEventTypes drives --list-event-types and validates --event-type
// values up front so all bad values can be reported in one error. Sourced from
// SDK enum constants so a rename or removal in go-auth0 is a compile-time
// failure here. New types added to the SDK still need a manual append.
var supportedEventTypes = []string{
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupCreated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupMemberAdded),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupMemberDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupRoleAssigned),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupRoleDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumGroupUpdated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationConnectionAdded),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationConnectionRemoved),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationConnectionUpdated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationCreated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationGroupRoleAssigned),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationGroupRoleDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationMemberAdded),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationMemberDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationMemberRoleAssigned),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationMemberRoleDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumOrganizationUpdated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumUserCreated),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumUserDeleted),
	string(managementv2.EventStreamSubscribeEventsEventTypeEnumUserUpdated),
}

type subscribeInputs struct {
	From           string
	FromTimestamp  string
	EventTypes     []string
	Verbose        bool
	ShowHeartbeats bool
	OutputFile     string
	NoReconnect    bool
	MaxReconnects  int
	ListEventTypes bool
}

func subscribeEventStreamCmd(cli *cli) *cobra.Command {
	var inputs subscribeInputs

	cmd := &cobra.Command{
		Use:   "subscribe",
		Args:  cobra.NoArgs,
		Short: "Subscribe to live events via Server-Sent Events (SSE)",
		Long: "Subscribe to events emitted by your tenant via Server-Sent Events (SSE).\n\n" +
			"By default, every received event is rendered as a single, color-coded summary line:\n" +
			"  TIME  TYPE  SOURCE  EVENT-ID\n\n" +
			"Use --verbose to also print the full JSON payload after each summary, " +
			"or --json / --json-compact to emit raw JSON suitable for piping into `jq`.\n\n" +
			"Heartbeat (`offset-only`) messages are suppressed by default and surfaced via " +
			"a periodic faint indicator and a final cursor on disconnect; pass --show-heartbeats " +
			"to render each one.\n\n" +
			"The server rotates long-lived connections every few minutes; the command " +
			"transparently resumes from the last cursor so the session stays continuous. " +
			"Use --no-reconnect to exit when the connection ends, or --max-reconnects to cap " +
			"the number of consecutive failed reconnect attempts. Press Ctrl+C to disconnect; " +
			"a per-type summary and the latest cursor are printed so you can resume with --from.\n\n" +
			"Run with --list-event-types to print every value accepted by --event-type.",
		Example: `  auth0 event-streams subscribe
  auth0 event-streams subscribe --list-event-types
  auth0 event-streams subscribe --event-type user.created
  auth0 event-streams subscribe --event-type user.created --event-type user.updated
  auth0 event-streams subscribe --from-timestamp 2026-05-01T00:00:00Z
  auth0 event-streams subscribe --from <cursor>
  auth0 event-streams subscribe -v
  auth0 event-streams subscribe --show-heartbeats
  auth0 event-streams subscribe --output-file events.jsonl
  auth0 event-streams subscribe --json | jq .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.ListEventTypes {
				cli.renderer.Infof(ansi.Bold("Supported event types:"))
				for _, t := range supportedEventTypes {
					cli.renderer.Output("  " + t)
				}
				return nil
			}

			req := &managementv2.SubscribeEventsRequestParameters{}

			if inputs.From != "" {
				req.From = &inputs.From
			}
			if inputs.FromTimestamp != "" {
				req.FromTimestamp = &inputs.FromTimestamp
			}
			if len(inputs.EventTypes) > 0 {
				eventTypes := make([]*managementv2.EventStreamSubscribeEventsEventTypeEnum, 0, len(inputs.EventTypes))
				var invalid []string
				for _, t := range inputs.EventTypes {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					enum, err := managementv2.NewEventStreamSubscribeEventsEventTypeEnumFromString(t)
					if err != nil {
						invalid = append(invalid, t)
						continue
					}
					eventTypes = append(eventTypes, enum.Ptr())
				}
				if len(invalid) > 0 {
					return invalidEventTypesError(invalid)
				}
				req.EventType = eventTypes
			}

			var outFile *os.File
			if inputs.OutputFile != "" {
				f, err := os.OpenFile(inputs.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("failed to open --output-file %q: %w", inputs.OutputFile, err)
				}
				defer func() { _ = f.Close() }()
				outFile = f
			}

			useJSON := cli.json || cli.jsonCompact
			if !useJSON {
				cli.renderer.Infof(ansi.Faint("Subscribed to event stream. Press Ctrl+C to disconnect."))
			}

			counts := map[string]int{}
			var (
				totalEvents     uint64
				errorEvents     uint64
				heartbeats      uint64
				lastOffset      string
				lastHeartbeatAt time.Time
				countsMu        sync.Mutex
				summaryOnce     sync.Once
				userInterrupted atomic.Bool
				startedAt       = time.Now()
			)

			flushSummary := func() {
				summaryOnce.Do(func() {
					if useJSON {
						return
					}
					countsMu.Lock()
					defer countsMu.Unlock()

					total := atomic.LoadUint64(&totalEvents)
					errs := atomic.LoadUint64(&errorEvents)
					hbs := atomic.LoadUint64(&heartbeats)
					duration := time.Since(startedAt).Round(time.Second)

					cli.renderer.Newline()
					cli.renderer.Infof(ansi.Bold("Disconnected. Summary:"))
					cli.renderer.Infof("  Duration:        %s", duration)
					cli.renderer.Infof("  Events received: %d", total)
					if errs > 0 {
						cli.renderer.Infof("  Errors:          %s", ansi.BrightRed(fmt.Sprintf("%d", errs)))
					}
					cli.renderer.Infof("  Heartbeats:      %d", hbs)
					if len(counts) > 0 {
						types := make([]string, 0, len(counts))
						maxLen := 0
						for t := range counts {
							types = append(types, t)
							if len(t) > maxLen {
								maxLen = len(t)
							}
						}
						// Sort by count desc; tie-break alphabetically so output is stable.
						sort.Slice(types, func(i, j int) bool {
							if counts[types[i]] != counts[types[j]] {
								return counts[types[i]] > counts[types[j]]
							}
							return types[i] < types[j]
						})
						cli.renderer.Newline()
						cli.renderer.Infof("  By type:")
						for _, t := range types {
							cli.renderer.Infof("    %s   %d", padRight(t, maxLen), counts[t])
						}
					}
					if lastOffset != "" {
						cli.renderer.Newline()
						cli.renderer.Infof("Resume with: %s", ansi.Cyan(fmt.Sprintf("auth0 event-streams subscribe --from %s", lastOffset)))
					}
				})
			}

			// The root command installs a SIGINT handler that calls os.Exit(0)
			// from a goroutine, which would skip our deferred summary. Reset
			// it first so only our handler runs: it cancels the stream context
			// so the resume loop unwinds cleanly, prints the summary, and exits.
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			signal.Reset(os.Interrupt, syscall.SIGTERM)
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)
			go func() {
				if _, ok := <-sigCh; !ok {
					return
				}
				userInterrupted.Store(true)
				cancel()
			}()

			// Per-event handler shared across reconnects. Returns true when an
			// event counts as forward progress (a real event or heartbeat),
			// which resets the reconnect backoff.
			handleEvent := func(event managementv2.EventStreamSubscribeEventsResponseContent) bool {
				summary := summarizeEvent(&event)

				if summary.offset != "" {
					lastOffset = summary.offset
				}

				// Connection_timeout is emitted by the server right before it
				// drops the SSE connection; we resume from the cursor, so it's
				// a protocol artifact and must not surface in any output sink.
				if summary.isError && summary.errorCode == "connection_timeout" {
					return false
				}

				// Always persist raw payload to file if requested.
				if outFile != nil {
					raw, mErr := json.Marshal(&event)
					if mErr == nil {
						_, _ = outFile.Write(raw)
						_, _ = outFile.WriteString("\n")
					}
				}

				if useJSON {
					if cli.jsonCompact {
						cli.renderer.JSONCompactResult(&event)
					} else {
						cli.renderer.JSONResult(&event)
					}
					return true
				}

				if summary.isHeartbeat {
					atomic.AddUint64(&heartbeats, 1)
					if inputs.ShowHeartbeats {
						renderHeartbeatLine(cli, summary)
					} else if time.Since(lastHeartbeatAt) > 30*time.Second {
						// Periodic faint pulse so users know the connection is alive.
						lastHeartbeatAt = time.Now()
						renderHeartbeatPulse(cli, summary)
					}
					return true
				}

				if summary.isError {
					atomic.AddUint64(&errorEvents, 1)
					renderErrorEvent(cli, summary)
				} else {
					atomic.AddUint64(&totalEvents, 1)
					countsMu.Lock()
					counts[summary.eventType]++
					countsMu.Unlock()
					renderEventSummary(cli, summary)
				}

				if inputs.Verbose {
					payload, mErr := json.MarshalIndent(&event, "", "  ")
					if mErr != nil {
						cli.renderer.Warnf("failed to marshal event payload: %v", mErr)
					} else {
						cli.renderer.Output(ansi.ColorizeJSON(string(payload)))
					}
				}
				return true
			}

			// Outer resume loop. The server drops long-lived SSE connections
			// every few minutes by design; rather than treating that as the
			// end of the stream, we transparently re-subscribe from the last
			// cursor so the session feels continuous (the same model used by
			// `stripe listen`, `kubectl logs -f`, and `aws logs tail --follow`).
			//
			// Backoff only kicks in for consecutive failures that made zero
			// progress. Any forward progress resets it, so a healthy stream
			// that simply rotates connections reconnects instantly. We give up
			// only after --max-reconnects consecutive zero-progress failures
			// (0 = retry indefinitely while progress is being made).
			consecutiveFailures := 0
			for {
				if ctx.Err() != nil {
					flushSummary()
					return nil
				}

				progressed, serverRetry, err := runStreamSession(ctx, cli.apiv2, req, lastOffset, handleEvent)

				// Ctrl+C or parent cancellation: always a graceful exit.
				if userInterrupted.Load() || errors.Is(err, context.Canceled) || ctx.Err() != nil {
					flushSummary()
					return nil
				}

				if inputs.NoReconnect {
					if err != nil {
						return fmt.Errorf("error receiving event: %w", err)
					}
					flushSummary()
					return nil
				}

				// A clean end-of-stream (err == nil) or any session that made
				// forward progress resets the backoff so connection rotations
				// reconnect instantly. Only consecutive zero-progress failures
				// accrue toward the --max-reconnects ceiling.
				if err == nil || progressed {
					consecutiveFailures = 0
				} else {
					consecutiveFailures++
				}

				if inputs.MaxReconnects > 0 && consecutiveFailures > inputs.MaxReconnects {
					flushSummary()
					if !useJSON {
						cli.renderer.Errorf("Giving up after %d consecutive reconnect attempts without progress.", inputs.MaxReconnects)
					}
					return fmt.Errorf("stream reconnection failed after %d consecutive attempts: %w", inputs.MaxReconnects, err)
				}

				delay := reconnectBackoff(consecutiveFailures, serverRetry)
				// Healthy connection rotations resume silently (even when the
				// server asks for a short `retry:` wait) so a normal long-lived
				// session looks seamless. Only surface a notice once we're
				// actually backing off real, consecutive failures.
				if !useJSON && consecutiveFailures > 0 {
					cli.renderer.Infof(ansi.Faint(fmt.Sprintf("· connection lost, reconnecting in %s…", delay.Round(time.Millisecond))))
				}

				select {
				case <-ctx.Done():
					flushSummary()
					return nil
				case <-time.After(delay):
				}
			}
		},
	}

	eventSubscribeFrom.RegisterString(cmd, &inputs.From, "")
	eventSubscribeFromTimestamp.RegisterString(cmd, &inputs.FromTimestamp, "")
	eventSubscribeEventType.RegisterStringSlice(cmd, &inputs.EventTypes, nil)
	eventSubscribeVerbose.RegisterBool(cmd, &inputs.Verbose, false)
	eventSubscribeShowHeartbeats.RegisterBool(cmd, &inputs.ShowHeartbeats, false)
	eventSubscribeOutputFile.RegisterString(cmd, &inputs.OutputFile, "")
	eventSubscribeNoReconnect.RegisterBool(cmd, &inputs.NoReconnect, false)
	eventSubscribeMaxReconnects.RegisterInt(cmd, &inputs.MaxReconnects, 0)
	eventSubscribeListEventTypes.RegisterBool(cmd, &inputs.ListEventTypes, false)
	cmd.MarkFlagsMutuallyExclusive("no-reconnect", "max-reconnects")

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output each event as JSON (one indented object per event).")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output each event as compact, single-line JSON (newline-delimited).")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// runStreamSession opens a single SSE subscription and pumps events through
// handleEvent until the connection ends or ctx is cancelled. SDK-internal
// reconnection is disabled so this function owns exactly one connection; the
// caller's outer loop is the single source of truth for reconnection. When
// resumeFrom is set it overrides req.From so each reconnect picks up exactly
// where the last delivered event left off. It returns whether the session made
// any forward progress (used to reset the caller's backoff), the server's most
// recently advertised SSE `retry:` interval (0 if none) so the caller never
// reconnects faster than the server asked, and the terminating error (nil on a
// clean server close).
func runStreamSession(
	ctx context.Context,
	api *auth0.APIV2,
	req *managementv2.SubscribeEventsRequestParameters,
	resumeFrom string,
	handleEvent func(managementv2.EventStreamSubscribeEventsResponseContent) bool,
) (progressed bool, serverRetry time.Duration, err error) {
	if resumeFrom != "" {
		req.From = &resumeFrom
	}

	stream, err := api.Events.Subscribe(ctx, req, managementoption.WithoutStreamReconnection())
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = stream.Close() }()

	for {
		event, recvErr := stream.Recv()
		if recvErr != nil {
			// The server may advertise an SSE `retry:` directive telling
			// clients how long to wait before reconnecting; honor it as a
			// floor so we respect server-side load shedding.
			retry := time.Duration(stream.LastRetryMs()) * time.Millisecond
			// EOF means the server closed the stream normally (including the
			// periodic connection rotation); surface it as a clean end so the
			// caller resumes without counting it as a failure.
			if errors.Is(recvErr, io.EOF) {
				return progressed, retry, nil
			}
			return progressed, retry, recvErr
		}
		if handleEvent(event) {
			progressed = true
		}
	}
}

// reconnectBackoff returns the delay before the next reconnect attempt. It is
// the larger of the server-advertised SSE `retry:` interval (serverRetry) and
// our own exponential backoff with full jitter, so we never reconnect faster
// than the server asked. With no consecutive failures and no server directive
// the delay is 0 (a healthy connection rotation reconnects instantly). Each
// subsequent consecutive failure doubles the jitter ceiling up to
// reconnectMaxBackoff; the actual jitter is a random value in [0, ceiling) to
// avoid thundering-herd reconnects across many clients.
func reconnectBackoff(consecutiveFailures int, serverRetry time.Duration) time.Duration {
	var jittered time.Duration
	if consecutiveFailures > 0 {
		ceiling := reconnectBaseBackoff << (consecutiveFailures - 1)
		if ceiling > reconnectMaxBackoff || ceiling <= 0 {
			ceiling = reconnectMaxBackoff
		}
		jittered = time.Duration(rand.Int64N(int64(ceiling)))
	}
	if serverRetry > jittered {
		return serverRetry
	}
	return jittered
}

const (
	reconnectBaseBackoff = 500 * time.Millisecond
	reconnectMaxBackoff  = 30 * time.Second
)

// eventSummary is a generic, payload-agnostic projection of an SSE message
// extracted by re-marshalling the SDK union type and pulling the standard
// CloudEvents envelope fields.
type eventSummary struct {
	eventType    string
	isHeartbeat  bool
	isError      bool
	offset       string
	id           string
	source       string
	time         time.Time
	errorCode    string
	errorMessage string
}

func summarizeEvent(ev *managementv2.EventStreamSubscribeEventsResponseContent) eventSummary {
	s := eventSummary{eventType: ev.GetType()}
	if s.eventType == "offset-only" {
		s.isHeartbeat = true
		if oo := ev.GetOffsetOnly(); oo != nil {
			s.offset = oo.GetOffset()
		}
		return s
	}

	if s.eventType == "error" {
		s.isError = true
		if em := ev.GetError(); em != nil {
			if d := em.GetError(); d != nil {
				s.errorCode = string(d.GetCode())
				s.errorMessage = d.GetMessage()
				if o := d.Offset; o != nil {
					s.offset = *o
				}
			}
		}
		return s
	}

	// All concrete event payloads share the same shape:
	//   { "type": "...", "offset": "...", "event": { "id", "time", "source", ... } }
	// Re-marshal once and extract the envelope generically so we don't have
	// to switch on every event variant.
	raw, err := json.Marshal(ev)
	if err != nil {
		return s
	}
	var envelope struct {
		Offset string `json:"offset"`
		Event  struct {
			ID     string    `json:"id"`
			Source string    `json:"source"`
			Time   time.Time `json:"time"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return s
	}
	s.offset = envelope.Offset
	s.id = envelope.Event.ID
	s.source = envelope.Event.Source
	s.time = envelope.Event.Time
	return s
}

// Column widths for the rendered event line. Padded before coloring so that
// ANSI escape sequences don't throw off visual alignment. Values that exceed
// the width are kept full-length (we never truncate forensic data like event
// IDs or sources); only that one row jitters.
const (
	eventTypeColWidth   = 32
	eventSourceColWidth = 32
)

func renderEventSummary(cli *cli, s eventSummary) {
	ts := s.time
	if ts.IsZero() {
		ts = time.Now()
	}
	line := fmt.Sprintf(
		"%s  %s  %s  %s",
		ansi.Faint(ts.Local().Format("15:04:05")),
		ansi.Bold(colorForEventType(padRight(s.eventType, eventTypeColWidth))),
		padRight(s.source, eventSourceColWidth),
		ansi.Faint(s.id),
	)
	cli.renderer.Output(line)
}

func renderErrorEvent(cli *cli, s eventSummary) {
	ts := time.Now()
	line := fmt.Sprintf(
		"%s  %s  %s  %s",
		ansi.Faint(ts.Local().Format("15:04:05")),
		ansi.Bold(colorForEventType(padRight("error", eventTypeColWidth))),
		ansi.BrightRed(padRight(s.errorCode, eventSourceColWidth)),
		s.errorMessage,
	)
	cli.renderer.Output(line)
}

// padRight pads s with spaces to width, or returns s unchanged if it already
// exceeds width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// invalidEventTypesError builds a user-friendly error for one or more unknown
// --event-type values and points to --list-event-types for the full list. The
// SDK's underlying error mentions internal Go type names, so we don't surface
// it.
func invalidEventTypesError(values []string) error {
	var b strings.Builder
	if len(values) == 1 {
		fmt.Fprintf(&b, "invalid --event-type value %q", values[0])
	} else {
		quoted := make([]string, 0, len(values))
		for _, v := range values {
			quoted = append(quoted, fmt.Sprintf("%q", v))
		}
		fmt.Fprintf(&b, "invalid --event-type values: %s", strings.Join(quoted, ", "))
	}
	b.WriteString("\nRun `auth0 event-streams subscribe --list-event-types` to see all supported types")
	return errors.New(b.String())
}

func renderHeartbeatLine(cli *cli, s eventSummary) {
	cli.renderer.Output(fmt.Sprintf(
		"%s  %s  %s",
		ansi.Faint(time.Now().Local().Format("15:04:05")),
		ansi.Faint("heartbeat"),
		ansi.Faint(shortOffset(s.offset)),
	))
}

func renderHeartbeatPulse(cli *cli, s eventSummary) {
	cli.renderer.Output(ansi.Faint(fmt.Sprintf(
		"· still listening (cursor %s)",
		shortOffset(s.offset),
	)))
}

func shortOffset(o string) string {
	if len(o) <= 12 {
		return o
	}
	return o[:8] + "…" + o[len(o)-4:]
}

func colorForEventType(t string) string {
	switch {
	case strings.HasSuffix(t, ".created") || strings.HasSuffix(t, ".added") || strings.HasSuffix(t, ".assigned"):
		return ansi.Green(t)
	case strings.HasSuffix(t, ".updated"):
		return ansi.Cyan(t)
	case strings.HasSuffix(t, ".deleted") || strings.HasSuffix(t, ".removed"):
		return ansi.Red(t)
	case t == "error":
		return ansi.BrightRed(t)
	default:
		return t
	}
}
