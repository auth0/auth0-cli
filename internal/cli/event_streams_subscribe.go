package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	managementv2 "github.com/auth0/go-auth0/v2/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
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
)

type subscribeInputs struct {
	From           string
	FromTimestamp  string
	EventTypes     []string
	Verbose        bool
	ShowHeartbeats bool
	OutputFile     string
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
			"to render each one. Press Ctrl+C to disconnect; a per-type summary and the " +
			"latest cursor will be printed so you can resume with --from.",
		Example: `  auth0 event-streams subscribe
  auth0 event-streams subscribe --event-type user.created
  auth0 event-streams subscribe --event-type user.created --event-type user.updated
  auth0 event-streams subscribe --from-timestamp 2026-05-01T00:00:00Z
  auth0 event-streams subscribe --from <cursor>
  auth0 event-streams subscribe -v
  auth0 event-streams subscribe --show-heartbeats
  auth0 event-streams subscribe --output-file events.jsonl
  auth0 event-streams subscribe --json | jq .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &managementv2.SubscribeEventsRequestParameters{}

			if inputs.From != "" {
				req.From = &inputs.From
			}
			if inputs.FromTimestamp != "" {
				req.FromTimestamp = &inputs.FromTimestamp
			}
			if len(inputs.EventTypes) > 0 {
				eventTypes := make([]*managementv2.EventStreamSubscribeEventsEventTypeEnum, 0, len(inputs.EventTypes))
				for _, t := range inputs.EventTypes {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					enum, err := managementv2.NewEventStreamSubscribeEventsEventTypeEnumFromString(t)
					if err != nil {
						return fmt.Errorf("invalid --event-type value %q: %w", t, err)
					}
					eventTypes = append(eventTypes, enum.Ptr())
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

			stream, err := cli.apiv2.Events.Subscribe(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("failed to subscribe to events: %w", err)
			}
			defer func() { _ = stream.Close() }()

			useJSON := cli.json || cli.jsonCompact
			if !useJSON {
				cli.renderer.Infof(ansi.Faint("Subscribed to event stream. Press Ctrl+C to disconnect."))
			}

			counts := map[string]int{}
			var (
				totalEvents     uint64
				heartbeats      uint64
				lastOffset      string
				lastHeartbeatAt time.Time
				countsMu        sync.Mutex
				summaryOnce     sync.Once
				streamClosed    atomic.Bool
			)

			flushSummary := func() {
				summaryOnce.Do(func() {
					if useJSON {
						return
					}
					countsMu.Lock()
					defer countsMu.Unlock()

					cli.renderer.Newline()
					cli.renderer.Infof(ansi.Bold("Disconnected. Summary:"))
					cli.renderer.Infof("  Events received: %d", atomic.LoadUint64(&totalEvents))
					cli.renderer.Infof("  Heartbeats:      %d", atomic.LoadUint64(&heartbeats))
					if len(counts) > 0 {
						types := make([]string, 0, len(counts))
						for t := range counts {
							types = append(types, t)
						}
						sort.Strings(types)
						for _, t := range types {
							cli.renderer.Infof("  %s %s %d", ansi.Faint("·"), t, counts[t])
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
			// it first so only our handler runs, then print the summary and
			// exit cleanly ourselves.
			signal.Reset(os.Interrupt, syscall.SIGTERM)
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)
			go func() {
				if _, ok := <-sigCh; !ok {
					return
				}
				streamClosed.Store(true)
				_ = stream.Close()
				flushSummary()
				os.Exit(0)
			}()

			for {
				event, err := stream.Recv()
				if err != nil {
					// Treat as graceful shutdown if:
					// - EOF (server closed)
					// - context cancelled
					// - we closed the stream ourselves (Ctrl+C)
					// - http2 body closed error (result of stream.Close())
					isGraceful := errors.Is(err, io.EOF) ||
						errors.Is(err, cmd.Context().Err()) ||
						cmd.Context().Err() != nil ||
						streamClosed.Load() ||
						strings.Contains(err.Error(), "response body closed")
					if isGraceful {
						flushSummary()
						return nil
					}
					return fmt.Errorf("error receiving event: %w", err)
				}

				summary := summarizeEvent(&event)

				if summary.offset != "" {
					lastOffset = summary.offset
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
					continue
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
					continue
				}

				atomic.AddUint64(&totalEvents, 1)
				countsMu.Lock()
				counts[summary.eventType]++
				countsMu.Unlock()

				renderEventSummary(cli, summary)

				if inputs.Verbose {
					payload, mErr := json.MarshalIndent(&event, "", "  ")
					if mErr != nil {
						cli.renderer.Warnf("failed to marshal event payload: %v", mErr)
					} else {
						cli.renderer.Output(ansi.ColorizeJSON(string(payload)))
					}
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

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output each event as JSON (one indented object per event).")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output each event as compact, single-line JSON (newline-delimited).")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// eventSummary is a generic, payload-agnostic projection of an SSE message
// extracted by re-marshalling the SDK union type and pulling the standard
// CloudEvents envelope fields.
type eventSummary struct {
	eventType   string
	isHeartbeat bool
	offset      string
	id          string
	source      string
	time        time.Time
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

func renderEventSummary(cli *cli, s eventSummary) {
	ts := s.time
	if ts.IsZero() {
		ts = time.Now()
	}
	line := fmt.Sprintf(
		"%s  %s  %s  %s",
		ansi.Faint(ts.Local().Format("15:04:05")),
		ansi.Bold(colorForEventType(s.eventType)),
		s.source,
		ansi.Faint(s.id),
	)
	cli.renderer.Output(line)
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
