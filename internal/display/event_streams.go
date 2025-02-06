package display

import (
	"encoding/json"
	"strings"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type eventStreamView struct {
	ID            string
	Name          string
	Type          string
	Status        string
	Subscriptions []string
	Configuration string
	raw           interface{}
}

func (v *eventStreamView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Status", "Subscriptions", "Configuration"}
}

func (v *eventStreamView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Type, v.Status, strings.Join(v.Subscriptions, ", "), v.Configuration}
}

func (v *eventStreamView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"STATUS", v.Status},
		{"SUBSCRIPTIONS", strings.Join(v.Subscriptions, ", ")},
		{"CONFIGURATION", v.Configuration},
	}
}

func (v *eventStreamView) Object() interface{} {
	return v.raw
}

func (r *Renderer) EventStreamsList(eventStreams []*management.EventStream) {
	resource := "event streams"

	r.Heading(resource)

	if len(eventStreams) == 0 {
		r.EmptyState(resource, "Use 'auth0 events create' to add one")
		return
	}

	var res []View
	for _, e := range eventStreams {
		res = append(res, makeEventStreamView(e))
	}

	r.Results(res)
}

func (r *Renderer) EventStreamShow(eventStream *management.EventStream) {
	r.Heading("eventStream")
	r.Result(makeEventStreamView(eventStream))
}

func (r *Renderer) EventStreamCreate(eventStream *management.EventStream) {
	r.Heading("eventStream created")
	r.Result(makeEventStreamView(eventStream))
}

func (r *Renderer) EventStreamUpdate(eventStream *management.EventStream) {
	r.Heading("eventStream updated")
	r.Result(makeEventStreamView(eventStream))
}

func makeEventStreamView(eventStream *management.EventStream) *eventStreamView {
	var subscriptions []string
	for _, subs := range eventStream.GetSubscriptions() {
		subscriptions = append(subscriptions, subs.GetEventStreamSubscriptionType())
	}

	return &eventStreamView{
		ID:            ansi.Faint(eventStream.GetID()),
		Name:          eventStream.GetName(),
		Type:          eventStream.Destination.GetEventStreamDestinationType(),
		Status:        eventStream.GetStatus(),
		Subscriptions: subscriptions,
		Configuration: formatConfiguration(eventStream.GetDestination().GetEventStreamDestinationConfiguration()),
		raw:           eventStream,
	}
}

func formatConfiguration(cfg map[string]interface{}) string {
	if cfg == nil {
		return ""
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(raw)
}
