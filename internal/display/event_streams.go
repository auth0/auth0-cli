package display

import (
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

func (r *Renderer) EventStreamsList(eventStreams []*management.EventStream) error {
	resource := "event streams"

	r.Heading(resource)

	if len(eventStreams) == 0 {
		r.EmptyState(resource, "Use 'auth0 events create' to add one")
		return nil
	}

	var res []View
	for _, e := range eventStreams {
		view, err := makeEventStreamView(e)
		if err != nil {
			return err
		}

		res = append(res, view)
	}

	r.Results(res)

	return nil
}

func (r *Renderer) EventStreamShow(eventStream *management.EventStream) error {
	r.Heading("eventStream")

	view, err := makeEventStreamView(eventStream)
	if err != nil {
		return err
	}

	r.Result(view)

	return nil
}

func (r *Renderer) EventStreamCreate(eventStream *management.EventStream) error {
	r.Heading("eventStream created")

	view, err := makeEventStreamView(eventStream)
	if err != nil {
		return err
	}

	r.Result(view)

	return nil
}

func (r *Renderer) EventStreamUpdate(eventStream *management.EventStream) error {
	r.Heading("eventStream updated")

	view, err := makeEventStreamView(eventStream)
	if err != nil {
		return err
	}

	r.Result(view)

	return nil
}

func makeEventStreamView(eventStream *management.EventStream) (*eventStreamView, error) {
	var subscriptions []string
	for _, subs := range eventStream.GetSubscriptions() {
		subscriptions = append(subscriptions, subs.GetEventStreamSubscriptionType())
	}

	configuration, err := toJSONString(eventStream.GetDestination().GetEventStreamDestinationConfiguration())
	if err != nil {
		return nil, err
	}

	return &eventStreamView{
		ID:            ansi.Faint(eventStream.GetID()),
		Name:          eventStream.GetName(),
		Type:          eventStream.Destination.GetEventStreamDestinationType(),
		Status:        eventStream.GetStatus(),
		Subscriptions: subscriptions,
		Configuration: configuration,
		raw:           eventStream,
	}, nil
}
