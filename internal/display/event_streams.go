package display

import (
	"fmt"
	"github.com/auth0/go-auth0/management"
	"github.com/manifoldco/promptui"
	"github.com/mattn/go-tty"
	"strings"
	"time"

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

/*----------------------------------------------- Delivery ------------------------------------------------.*/

type eventDeliveryView struct {
	Delivery *management.EventDelivery
}

func (v *eventDeliveryView) AsTableHeader() []string {
	return []string{"ID", "Event Type", "Status", "Attempts"}
}

func (v *eventDeliveryView) AsTableRow() []string {
	d := v.Delivery
	return []string{
		ansi.Faint(d.GetID()),
		d.GetEventType(),
		d.GetStatus(),
		fmt.Sprintf("%d", len(d.Attempts)),
	}
}

func (v *eventDeliveryView) KeyValues() [][]string {
	d := v.Delivery
	return [][]string{
		{"ID", ansi.Faint(d.GetID())},
		{"EVENT TYPE", d.GetEventType()},
		{"STATUS", d.GetStatus()},
		{"ATTEMPTS", fmt.Sprintf("%d", len(d.Attempts))},
	}
}

func (v *eventDeliveryView) Object() interface{} {
	return v.Delivery
}

func (v *eventDeliveryView) AsPromptHeaderString() string {
	row := v.AsTableHeader()
	return fmt.Sprintf(
		"    "+"\033[4m%-*s  %-*s  %-*s  %-*s\033[0m",
		27, row[0],
		13, row[1],
		6, row[2],
		0, row[3],
	)
}

func (v *eventDeliveryView) AsPromptRowString() string {
	d := v.Delivery
	return fmt.Sprintf(
		"%s\t%s\t%s\t%d",
		ansi.Faint(d.GetID()),
		d.GetEventType(),
		d.GetStatus(),
		len(d.Attempts),
	)
}

func (r *Renderer) EventDeliveriesList(deliveries []*management.EventDelivery) error {
	resource := "event deliveries"
	r.Heading(resource)

	var res []View
	for _, d := range deliveries {
		res = append(res, &eventDeliveryView{Delivery: d})
	}

	r.Results(res)
	return nil
}

func (r *Renderer) EventDeliveryPrompt(deliveries []*management.EventDelivery, currentIndex *int) *management.EventDelivery {
	resource := "event deliveries"
	r.Heading(resource)

	if len(deliveries) == 0 {
		r.Errorf("no deliveries found to select")
		return nil
	}

	header := &eventDeliveryView{Delivery: deliveries[0]}
	label := header.AsPromptHeaderString()

	var rows []string
	for _, d := range deliveries {
		view := &eventDeliveryView{Delivery: d}
		rows = append(rows, view.AsPromptRowString())
	}

	promptui.IconInitial = promptui.Styler()("")
	prompt := promptui.Select{
		Label:    label,
		Items:    rows,
		Size:     10,
		HideHelp: true,
		Stdout:   &noBellStdout{},
		Templates: &promptui.SelectTemplates{
			Label: "{{ . }}",
		},
	}

	var err error
	*currentIndex, _, err = prompt.RunCursorAt(*currentIndex, *currentIndex)
	if err != nil {
		return nil // Ctrl+C or Escape.
	}
	return deliveries[*currentIndex]
}

/*----------------------------------------------- Show Delivery ------------------------------------------------.*/

func (r *Renderer) ShowDelivery(delivery *management.EventDelivery) {
	if r.Format == OutputFormatJSON {
		r.JSONResult(delivery)
		return
	}
	r.RenderDeliveryMetadata(delivery)
	r.RenderDeliveryAttempts(delivery.Attempts)

	if r.ConfirmPrompt("View event payload?") {
		fmt.Println("\nPayload used for the event:")
		r.JSONResult(delivery.GetEvent().GetData())
	}
}

/*----------------------------------------------- DeliveryMetaData ------------------------------------------------.*/

type eventDeliveryMetadataView struct {
	ID            string
	EventType     string
	StreamID      string
	Status        string
	AttemptsCount int
	raw           interface{}
}

func (v *eventDeliveryMetadataView) AsTableHeader() []string {
	return []string{"Field", "Value"}
}

func (v *eventDeliveryMetadataView) AsTableRow() []string {
	return []string{
		ansi.Faint("ID"), v.ID,
		ansi.Faint("Event Type"), v.EventType,
		ansi.Faint("Stream ID"), v.StreamID,
		ansi.Faint("Status"), v.Status,
		ansi.Faint("Attempts Count"), fmt.Sprintf("%d", v.AttemptsCount),
	}
}

func (v *eventDeliveryMetadataView) KeyValues() [][]string {
	return [][]string{
		{"ID", v.ID},
		{"Event Type", v.EventType},
		{"Stream ID", v.StreamID},
		{"Status", v.Status},
		{"Attempts Count", fmt.Sprintf("%d", v.AttemptsCount)},
	}
}

func (v *eventDeliveryMetadataView) Object() interface{} {
	return map[string]interface{}{
		"id":             v.ID,
		"event_type":     v.EventType,
		"stream_id":      v.StreamID,
		"status":         v.Status,
		"attempts_count": v.AttemptsCount,
	}
}

func makeEventDeliveryMetadataView(delivery *management.EventDelivery) *eventDeliveryMetadataView {
	return &eventDeliveryMetadataView{
		ID:            delivery.GetID(),
		EventType:     delivery.GetEventType(),
		StreamID:      delivery.GetEventStreamID(),
		Status:        delivery.GetStatus(),
		AttemptsCount: len(delivery.Attempts),
		raw:           delivery,
	}
}

func (r *Renderer) RenderDeliveryMetadata(delivery *management.EventDelivery) {
	r.Newline()
	fmt.Println(ansi.Bold(ansi.Cyan("Event Delivery Metadata")))
	r.Result(makeEventDeliveryMetadataView(delivery))
}

/*----------------------------------------------- Delivery Attempt ------------------------------------------------.*/

type deliveryAttemptView struct {
	Index     int
	Status    string
	Timestamp string
	Duration  string
	Error     string
	raw       interface{}
}

func (v *deliveryAttemptView) AsTableHeader() []string {
	return []string{
		"#", "Status", "Timestamp", "Duration", "Error"}
}

func (v *deliveryAttemptView) AsTableRow() []string {
	return []string{
		fmt.Sprintf("%d", v.Index),
		statusColor(v.Status),
		v.Timestamp,
		v.Duration,
		v.Error,
	}
}

func (v *deliveryAttemptView) Object() interface{} {
	return v.raw
}

func (r *Renderer) RenderDeliveryAttempts(attempts []*management.DeliveryAttempt) {
	r.Newline()
	fmt.Println(ansi.Bold(ansi.Cyan("Event Delivery Attempts")))

	var views []View
	for i, a := range attempts {
		duration := "-"
		if a.Duration != nil {
			duration = fmt.Sprintf("%.2fms", *a.Duration)
		}

		errorMessage := "-"
		if a.ErrorMessage != nil {
			errorMessage = *a.ErrorMessage
		}
		if len(errorMessage) > 60 {
			errorMessage = errorMessage[:60] + "..."
		}

		view := &deliveryAttemptView{
			Index:     i + 1,
			Status:    a.GetStatus(),
			Timestamp: a.GetTimestamp().String(),
			Duration:  duration,
			Error:     errorMessage,
			raw:       a,
		}
		views = append(views, view)
	}

	r.Results(views)
}

/*----------------------------------------------- Stats ------------------------------------------------.*/

const (
	MetricSuccessfulDeliveries = "auth0.event_streams.successful_deliveries"
	MetricFailedDeliveries     = "auth0.event_streams.failed_deliveries"
)

type eventStreamStatsView struct {
	ID       string
	Name     string
	From     string
	To       string
	Interval string
	raw      interface{}
}

func (v *eventStreamStatsView) AsTableHeader() []string {
	return []string{"Field", "Value"}
}

func (v *eventStreamStatsView) AsTableRow() []string {
	return []string{
		"ID", v.ID,
		"Name", v.Name,
		"From", v.From,
		"To", v.To,
		"Interval", v.Interval,
	}
}

func (v *eventStreamStatsView) KeyValues() [][]string {
	return [][]string{
		{"ID", v.ID},
		{"Name", v.Name},
		{"From", v.From},
		{"To", v.To},
		{"Interval", v.Interval},
	}
}

func (v *eventStreamStatsView) Object() interface{} {
	return v.raw
}

type eventStreamStatsRowView struct {
	Timestamp string
	Success   int
	Failure   int
	raw       interface{}
}

func (v *eventStreamStatsRowView) AsTableHeader() []string {
	return []string{"Timestamp", "Successful Deliveries", "Failed Deliveries"}
}

func (v *eventStreamStatsRowView) AsTableRow() []string {
	return []string{v.Timestamp, fmt.Sprintf("%d", v.Success), fmt.Sprintf("%d", v.Failure)}
}

func (v *eventStreamStatsRowView) KeyValues() [][]string {
	return [][]string{
		{"Timestamp", v.Timestamp},
		{"Successful Deliveries", fmt.Sprintf("%d", v.Success)},
		{"Failed Deliveries", fmt.Sprintf("%d", v.Failure)},
	}
}

func (v *eventStreamStatsRowView) Object() interface{} {
	return v.raw
}

func (r *Renderer) RenderEventStreamStats(stats *management.EventStreamStats) {

	if r.Format == OutputFormatJSON {
		r.JSONResult(stats)
		return
	}

	v := &eventStreamStatsView{
		ID:       stats.GetID(),
		Name:     stats.GetName(),
		From:     stats.Window.GetDateFrom().Format(time.RFC3339),
		To:       stats.Window.GetDateTo().Format(time.RFC3339),
		Interval: fmt.Sprintf("%ds", stats.Window.BucketInterval.GetScaleFactor()),
		raw:      stats,
	}

	r.Newline()
	fmt.Println(ansi.Bold(ansi.Cyan("Event Stream Stats")))
	r.Result(v)

	// Metric buckets
	metricMap := extractMetricMap(stats.Metrics)

	success := metricMap[MetricSuccessfulDeliveries]
	fail := metricMap[MetricFailedDeliveries]

	var rows []View
	for i, bucket := range stats.Buckets {
		timestamp := bucket.Format("2006-01-02 15:04")
		row := &eventStreamStatsRowView{
			Timestamp: timestamp,
			Success:   safeMetric(success, i),
			Failure:   safeMetric(fail, i),
			raw:       nil,
		}
		rows = append(rows, row)
	}

	r.Newline()
	fmt.Println(ansi.Bold(ansi.Cyan("Delivery Metrics")))
	r.Results(rows)

	// Metric Totals + Types
	r.Newline()
	if success != nil {
		s := findMetric(stats.Metrics, MetricSuccessfulDeliveries)
		if s != nil {
			fmt.Printf("  Successful Deliveries: Total = %d, Type = %s\n", s.GetWindowTotal(), s.GetType())
		}
	}
	if fail != nil {
		f := findMetric(stats.Metrics, MetricFailedDeliveries)
		if f != nil {
			fmt.Printf("  Failed Deliveries:     Total = %d, Type = %s\n", f.GetWindowTotal(), f.GetType())
		}
	}
}

func findMetric(metrics []*management.StatsMetric, name string) *management.StatsMetric {
	for _, m := range metrics {
		if m.GetName() == name {
			return m
		}
	}
	return nil
}

func extractMetricMap(metrics []*management.StatsMetric) map[string][]int {
	metricMap := make(map[string][]int)
	for _, metric := range metrics {
		values := make([]int, len(metric.Data))
		for i, v := range metric.Data {
			if v != nil {
				values[i] = *v
			} else {
				values[i] = 0
			}
		}
		metricMap[metric.GetName()] = values
	}
	return metricMap
}

func safeMetric(values []int, i int) int {
	if i >= 0 && i < len(values) {
		return values[i]
	}
	return 0
}

/*----------------------------------------------- Utils ------------------------------------------------.*/

func (r *Renderer) ConfirmPrompt(prompt string) bool {
	fmt.Printf("\n%s [y/N]: ", prompt)

	ContTty, _ := tty.Open()
	defer func(ContTty *tty.TTY) {
		_ = ContTty.Close()
	}(ContTty)

	rn, err := ContTty.ReadRune()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%c\n", rn)

	return rn == 'y' || rn == 'Y'
}

func statusColor(v string) string {
	switch strings.ToLower(v) {
	case "failed":
		return ansi.Red(v)
	case "retrying":
		return ansi.Yellow(v)
	case "success":
		return ansi.Green(v)
	default:
		return v
	}
}
