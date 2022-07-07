package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

const (
	eventNamePrefix   = "CLI"
	analyticsEndpoint = "https://heapanalytics.com/api/track"
	appID             = "1279799279"
	versionKey        = "version"
	osKey             = "os"
	archKey           = "arch"
)

type Tracker struct {
	wg sync.WaitGroup
}

type event struct {
	App        string            `json:"app_id"`
	ID         string            `json:"identity"`
	Event      string            `json:"event"`
	Timestamp  int64             `json:"timestamp"`
	Properties map[string]string `json:"properties"`
}

func NewTracker() *Tracker {
	return &Tracker{}
}

func (t *Tracker) TrackFirstLogin(id string) {
	eventName := fmt.Sprintf("%s - Auth0 - First Login", eventNamePrefix)
	t.track(eventName, id)
}

func (t *Tracker) TrackCommandRun(cmd *cobra.Command, id string) {
	eventName := generateRunEventName(cmd.CommandPath())
	t.track(eventName, id)
}

func (t *Tracker) Wait(ctx context.Context) {
	ch := make(chan struct{})

	go func() {
		t.wg.Wait()
		close(ch)
	}()

	select {
	case <-ch: // waitgroup is done
		return
	case <-ctx.Done():
		return
	}
}

func (t *Tracker) track(eventName string, id string) {
	if !shouldTrack() {
		return
	}

	event := newEvent(eventName, id)

	t.wg.Add(1)
	go t.sendEvent(event)
}

func (t *Tracker) sendEvent(event *event) {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", analyticsEndpoint, bytes.NewBuffer(jsonEvent))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		println(err.Error())
		return
	}

	// defers execute in LIFO order
	defer t.wg.Done()
	defer resp.Body.Close()
}

func newEvent(eventName string, id string) *event {
	return &event{
		App:       appID,
		ID:        id,
		Event:     eventName,
		Timestamp: timestamp(),
		Properties: map[string]string{
			versionKey: buildinfo.Version,
			osKey:      runtime.GOOS,
			archKey:    runtime.GOARCH,
		},
	}
}

func generateRunEventName(command string) string {
	return generateEventName(command, "Run")
}

func generateEventName(command string, action string) string {
	commands := strings.Split(command, " ")

	for i := range commands {
		commands[i] = cases.Title(language.English).String(commands[i])
	}

	if len(commands) == 1 { // the root command
		return fmt.Sprintf("%s - %s - %s", eventNamePrefix, commands[0], action)
	} else if len(commands) == 2 { // a top-level command e.g. auth0 apps
		return fmt.Sprintf("%s - %s - %s - %s", eventNamePrefix, commands[0], commands[1], action)
	} else if len(commands) >= 3 {
		return fmt.Sprintf("%s - %s - %s - %s", eventNamePrefix, commands[1], strings.Join(commands[2:], " "), action)
	}

	return eventNamePrefix
}

func shouldTrack() bool {
	if os.Getenv("AUTH0_CLI_ANALYTICS") == "false" || buildinfo.Version == "" { // Do not track debug builds
		return false
	}

	return true
}

func timestamp() int64 {
	t := time.Now()
	s := t.Unix() * 1e3
	ms := int64(t.Nanosecond()) / 1e6
	return s + ms
}
