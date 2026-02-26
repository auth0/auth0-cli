package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/auth0"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	eventStreamID = Argument{
		Name: "Id",
		Help: "Id of the Event Stream.",
	}

	eventStreamName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the Event Stream.",
		IsRequired: true,
	}

	eventStreamStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Status of the Event Stream. (enabled/disabled)",
	}

	eventStreamSubscriptions = Flag{
		Name:       "Subscriptions",
		LongForm:   "subscriptions",
		ShortForm:  "s",
		Help:       "Subscriptions of the Event Stream. Formatted as comma separated string. Eg. user.created,user.updated",
		IsRequired: true,
	}

	eventStreamType = Flag{
		Name:       "Type",
		LongForm:   "type",
		ShortForm:  "t",
		Help:       "Type of the Event Stream. Eg: webhook, eventbridge etc",
		IsRequired: true,
	}

	eventStreamConfig = Flag{
		Name:      "Configuration",
		LongForm:  "configuration",
		ShortForm: "c",
		Help: "Configuration of the Event Stream. Formatted as JSON. \n" +
			"Webhook Example: " + `{"webhook_endpoint":"https://my-webhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}} ` + "\n" +
			"Eventbridge Example: " + `{"aws_account_id":"7832467231933","aws_region":"us-east-2"}`,
		IsRequired: true,
	}

	eventPicker = Flag{
		Name:      "Interactive picker option on rendered events",
		LongForm:  "picker",
		ShortForm: "p",
		Help:      "Allows to toggle from list of events and view a selected event in detail",
	}
)

func eventStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-streams",
		Short: "Manage Event Stream",
		Long: "Events are a way for Auth0 customers to synchronize, correlate or orchestrate " +
			"changes that occur within Auth0 or 3rd-party identity providers to your app or 3rd party services.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listEventStreamsCmd(cli))
	cmd.AddCommand(createEventStreamCmd(cli))
	cmd.AddCommand(showEventStreamCmd(cli))
	cmd.AddCommand(updateEventStreamCmd(cli))
	cmd.AddCommand(deleteEventStreamCmd(cli))

	cmd.AddCommand(triggerEventStreamCmd(cli))
	cmd.AddCommand(deliveriesEventStreamCmd(cli))
	cmd.AddCommand(redeliverEventStreamCmd(cli))
	cmd.AddCommand(redeliverManyEventStreamCmd(cli))
	cmd.AddCommand(statsEventStreamCmd(cli))
	return cmd
}

func listEventStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your event streams",
		Long:    "List your existing event streams. To create one, run: `auth0 event-streams create`.",
		Example: `  auth0 event-streams list
  auth0 event-streams ls
  auth0 event-streams ls --json
  auth0 event-streams ls --json-compact
  auth0 event-streams ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list *management.EventStreamList

			if err := ansi.Waiting(func() (err error) {
				list, err = cli.api.EventStream.List(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to list event streams: %w", err)
			}

			return cli.renderer.EventStreamsList(list.EventStreams)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an event stream",
		Long:  "Display the name, type, status, subscriptions and other information about an event stream",
		Example: `  auth0 event-streams show
  auth0 event-streams show <event-id>
  auth0 event-streams show <event-id> --json
  auth0 event-streams show <event-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := eventStreamID.Pick(cmd, &inputs.ID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var eventStream *management.EventStream

			if err := ansi.Waiting(func() (err error) {
				eventStream, err = cli.api.EventStream.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read event stream with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.EventStreamShow(eventStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name          string
		Type          string
		Subscriptions []string
		Configuration string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new event stream",
		Long: "Create a new event stream.\n\n" +
			"To create interactively, use `auth0 event-streams create` with no flags.\n\n" +
			"To create non-interactively, supply the event stream name, type, subscriptions and configuration through the flags.",
		Example: `  auth0 event-streams create
  auth0 event-streams create --name my-event-stream --type eventbridge --subscriptions "user.created,user.updated" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 event-streams create --name my-event-stream --type webhook --subscriptions "user.created,user.deleted" --configuration '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
  auth0 event-streams create -n my-event-stream -t webhook -s "user.created,user.deleted" -c '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := eventStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if err := eventStreamType.Ask(cmd, &inputs.Type, nil); err != nil {
				return err
			}

			if err := eventStreamSubscriptions.AskMany(cmd, &inputs.Subscriptions, nil); err != nil {
				return err
			}

			var configuration map[string]interface{}

			if err := eventStreamConfig.Ask(cmd, &inputs.Configuration, auth0.String("{}")); err != nil {
				return err
			}

			if err := json.Unmarshal([]byte(inputs.Configuration), &configuration); err != nil {
				return fmt.Errorf("provider: %s event stream config invalid JSON: %w", inputs.Name, err)
			}

			if len(inputs.Configuration) == 0 {
				return fmt.Errorf("must provider configuration for event stream")
			}

			var subscriptions []management.EventStreamSubscription
			for _, sub := range inputs.Subscriptions {
				subscriptions = append(subscriptions, management.EventStreamSubscription{
					EventStreamSubscriptionType: &sub,
				})
			}
			eventStream := &management.EventStream{
				Name:          &inputs.Name,
				Subscriptions: &subscriptions,
				Destination: &management.EventStreamDestination{
					EventStreamDestinationType:          &inputs.Type,
					EventStreamDestinationConfiguration: configuration,
				},
			}

			if err := ansi.Waiting(func() error {
				return cli.api.EventStream.Create(cmd.Context(), eventStream)
			}); err != nil {
				return fmt.Errorf("failed to create event stream: %w", err)
			}

			return cli.renderer.EventStreamCreate(eventStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	eventStreamName.RegisterString(cmd, &inputs.Name, "")
	eventStreamType.RegisterString(cmd, &inputs.Type, "")
	eventStreamSubscriptions.RegisterStringSlice(cmd, &inputs.Subscriptions, nil)
	eventStreamConfig.RegisterString(cmd, &inputs.Configuration, "")

	return cmd
}

func updateEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		Name          string
		Status        string
		Subscriptions []string
		Configuration string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an event stream",
		Long: "Update an event stream.\n\n" +
			"To update interactively, use `auth0 event-streams update` with no arguments.\n\n" +
			"To update non-interactively, supply the event id, name, status, subscriptions and " +
			"configuration through the flags. An event stream type CANNOT be updated hence the configuration " +
			"should match the schema based on the type of event stream. Configuration for `eventbridge` streams " +
			"cannot be updated.",
		Example: `  auth0 event-streams update <event-id>
  auth0 event-streams update <event-id> --name my-event-stream
  auth0 event-streams update <event-id> --name my-event-stream --status enabled
  auth0 event-streams update <event-id> --name my-event-stream --status enabled --subscriptions "user.created,user.updated"
  auth0 event-streams update <event-id> --name my-event-stream --status disabled --subscriptions "user.deleted" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 event-streams update <event-id> --name my-event-stream --status enabled --subscriptions "user.created" --configuration '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"0909090909"}}
  auth0 event-streams update <event-id> -n my-event-stream --status enabled -s "user.created" -c '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"987654321"}}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := eventStreamID.Pick(cmd, &inputs.ID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			var oldEventStream *management.EventStream
			err := ansi.Waiting(func() (err error) {
				oldEventStream, err = cli.api.EventStream.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to read event stream with ID %q: %w", inputs.ID, err)
			}

			if e := eventStreamName.AskU(cmd, &inputs.Name, nil); e != nil {
				return e
			}
			if e := eventStreamStatus.AskU(cmd, &inputs.Status, nil); e != nil {
				return e
			}

			if e := eventStreamSubscriptions.AskManyU(cmd, &inputs.Subscriptions, nil); e != nil {
				return e
			}

			if e := eventStreamConfig.AskU(cmd, &inputs.Configuration, nil); e != nil {
				return e
			}

			updatedEventStream := &management.EventStream{}

			if inputs.Name != "" {
				updatedEventStream.Name = &inputs.Name
			}

			if inputs.Status != "" {
				updatedEventStream.Status = &inputs.Status
			}

			if len(inputs.Subscriptions) != 0 {
				var subscriptions []management.EventStreamSubscription
				for _, sub := range inputs.Subscriptions {
					subscriptions = append(subscriptions, management.EventStreamSubscription{
						EventStreamSubscriptionType: &sub,
					})
				}
				updatedEventStream.Subscriptions = &subscriptions
			}

			if inputs.Configuration != "" {
				var configuration map[string]interface{}
				if err := json.Unmarshal([]byte(inputs.Configuration), &configuration); err != nil {
					return fmt.Errorf("provider: %s event stream config invalid JSON: %w", inputs.Name, err)
				}
				updatedEventStream.Destination = &management.EventStreamDestination{
					EventStreamDestinationType:          auth0.String(oldEventStream.GetDestination().GetEventStreamDestinationType()),
					EventStreamDestinationConfiguration: configuration,
				}
			}

			if err = ansi.Waiting(func() error {
				return cli.api.EventStream.Update(cmd.Context(), oldEventStream.GetID(), updatedEventStream)
			}); err != nil {
				return fmt.Errorf("failed to update event stream with ID %q: %w", oldEventStream.GetID(), err)
			}

			return cli.renderer.EventStreamUpdate(updatedEventStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	eventStreamName.RegisterStringU(cmd, &inputs.Name, "")
	eventStreamStatus.RegisterStringU(cmd, &inputs.Status, "")
	eventStreamSubscriptions.RegisterStringSliceU(cmd, &inputs.Subscriptions, nil)
	eventStreamConfig.RegisterStringU(cmd, &inputs.Configuration, "")

	return cmd
}

func deleteEventStreamCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an event stream",
		Long: "Delete an event stream.\n\n" +
			"To delete interactively, use `auth0 event-streams delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the event id and the `--force` flag to skip confirmation.",
		Example: `  auth0 event-streams delete
  auth0 event-streams rm
  auth0 event-streams delete <event-id>
  auth0 event-streams delete <event-id> --force
  auth0 event-streams delete <event-id> <event-id2> <event-idn>
  auth0 event-streams delete <event-id> <event-id2> <event-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := eventStreamID.PickMany(cmd, &ids, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting event(s)", ids, func(i int, id string) error {
				if id != "" {
					if err := cli.api.EventStream.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete Event Stream with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func triggerEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID        string
		EventType string
		Payload   string
	}

	cmd := &cobra.Command{
		Use:   "trigger <event-id>",
		Args:  cobra.MaximumNArgs(1),
		Short: "Trigger a test event for an event stream",
		Long: "Manually trigger a test event for a specific Event Stream.\n\n" +
			"Use this to simulate an event like `user.created` or `user.updated`.\n" +
			"You can optionally provide a JSON payload from a file to simulate request content.",
		Example: `  auth0 event-streams trigger <event-id> --type user.created
  auth0 event-streams trigger <event-id> --type user.updated --payload ./test-event.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := eventStreamID.Pick(cmd, &inputs.ID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			if inputs.EventType == "" {
				var stream *management.EventStream
				if err := ansi.Waiting(func() (err error) {
					stream, err = cli.api.EventStream.Read(cmd.Context(), inputs.ID)
					return err
				}); err != nil {
					return fmt.Errorf("failed to fetch stream details for ID %q: %w", inputs.ID, err)
				}

				var eventTypeOptions []string
				for _, s := range stream.GetSubscriptions() {
					eventTypeOptions = append(eventTypeOptions, s.GetEventStreamSubscriptionType())
				}

				if err := eventStreamType.Select(cmd, &inputs.EventType, eventTypeOptions, nil); err != nil {
					return err
				}
			}

			var payload map[string]interface{}

			if inputs.Payload != "" {
				data, err := os.ReadFile(inputs.Payload)
				if err != nil {
					return fmt.Errorf("failed to read payload file %q: %w", inputs.Payload, err)
				}
				if err := json.Unmarshal(data, &payload); err != nil {
					return fmt.Errorf("invalid JSON in payload file: %w", err)
				}
			}

			testEvent := &management.TestEvent{
				EventType: auth0.String(inputs.EventType),
			}

			if payload != nil {
				testEvent.Data = payload
			}

			if err := ansi.Waiting(func() error {
				return cli.api.EventStream.Test(cmd.Context(), inputs.ID, testEvent)
			}); err != nil {
				return fmt.Errorf("failed to trigger test event: %w", err)
			}

			cli.renderer.Infof(ansi.Faint(fmt.Sprintf("✓ Triggered test event %q on stream %q with id: %q", inputs.EventType, inputs.ID, testEvent.GetID())))
			return nil
		},
	}

	cmd.Flags().StringVar(&inputs.EventType, "type", "", "Type of event to simulate (e.g., user.created) [required]")
	cmd.Flags().StringVar(&inputs.Payload, "payload", "", "Path to a JSON file with a custom payload (optional)")

	return cmd
}

func deliveriesEventStreamCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliveries",
		Short: "Manage event stream deliveries",
		Long:  "Inspect and monitor delivery attempts for triggered events in a given event stream.",
	}

	cmd.AddCommand(listDeliveriesCmd(cli))
	cmd.AddCommand(showDeliveryCmd(cli))

	return cmd
}

func listDeliveriesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID         string
		EventTypes []string
		Picker     bool
		From       string
		To         string
		N          int
	}

	cmd := &cobra.Command{
		Use:     "list [event-stream-id]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"ls"},
		Short:   "List failed deliveries for an event stream",
		Long: "List all failed delivery attempts associated with a specific event stream.\n" +
			"Optionally filter by event type(s) using the --type flag.",
		Example: `  auth0 event-streams deliveries list
  auth0 event-streams deliveries list <event-stream-id>
  auth0 event-streams deliveries list <event-stream-id> --type user.created
  auth0 event-streams deliveries list --json
  auth0 event-streams deliveries list --csv
  auth0 event-streams deliveries list --picker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := eventStreamID.Pick(cmd, &inputs.ID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			var opts []management.RequestOption

			if len(inputs.EventTypes) > 0 {
				opts = append(opts, management.WithEventTypes(inputs.EventTypes...))
			}

			opts = append(opts, management.Parameter("take", strconv.Itoa(inputs.N)))

			if inputs.From != "" {
				fromDate, err := parseFlexibleDate(inputs.From)
				if err != nil {
					return fmt.Errorf("invalid --from date: %w", err)
				}
				opts = append(opts, management.Parameter("date_from", fromDate))
			}

			if inputs.To != "" {
				toDate, err := parseFlexibleDate(inputs.To)
				if err != nil {
					return fmt.Errorf("invalid --to date: %w", err)
				}
				opts = append(opts, management.Parameter("date_to", toDate))
			}

			var deliveries *management.EventDeliveryList
			if err := ansi.Waiting(func() (err error) {
				deliveries, err = cli.api.EventStream.ListDeliveries(cmd.Context(), inputs.ID, opts...)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list deliveries for stream %q: %w", inputs.ID, err)
			}

			if len(deliveries.Deliveries) == 0 {
				fmt.Println("No deliveries found.")
				return nil
			}

			if !inputs.Picker {
				return cli.renderer.EventDeliveriesList(deliveries.Deliveries)
			}

			// Picker mode.
			var currentIndex = auth0.Int(0)
			for {
				selectedDelivery := cli.renderer.EventDeliveryPrompt(deliveries.Deliveries, currentIndex)
				if selectedDelivery == nil {
					return handleInputError(errors.New("bad input"))
				}
				cli.renderer.ShowDelivery(selectedDelivery)
				if cli.renderer.QuitPrompt() {
					break
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&inputs.EventTypes, "type", nil, "Filter deliveries by one or more event types (comma-separated)")

	cmd.Flags().StringVarP(&inputs.From, "from", "f", "", "Filter deliveries from this date (e.g. 2025-07-25, yesterday, -2d)")
	cmd.Flags().StringVarP(&inputs.To, "to", "t", "", "Filter deliveries up to this date (e.g. 2025-07-29, today)")
	cmd.Flags().IntVarP(&inputs.N, "n", "n", 50, "Number of results to return, defaults to 50")

	eventPicker.RegisterBool(cmd, &inputs.Picker, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in CSV format.")
	cmd.MarkFlagsMutuallyExclusive("json", "csv")

	return cmd
}

func showDeliveryCmd(cli *cli) *cobra.Command {
	var inputs struct {
		StreamID   string
		DeliveryID string
	}

	cmd := &cobra.Command{
		Use:   "show [stream-id] [delivery-id]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Show details for a specific delivery",
		Long: "Displays metadata, attempts, and event payload for a specific \n" +
			"delivery associated with an event stream. \n" +
			"If stream ID or delivery ID is not provided, you will be " +
			"prompted to select them interactively.",
		Example: `  auth0 event-streams deliveries show
  auth0 event-streams deliveries show <stream-id>
  auth0 event-streams deliveries show <stream-id> <delivery-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				inputs.StreamID = args[0]
			}
			if len(args) == 2 {
				inputs.DeliveryID = args[1]
			}

			if inputs.StreamID == "" {
				if err := eventStreamID.Pick(cmd, &inputs.StreamID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			if inputs.DeliveryID == "" {
				deliveries, err := fetchRecentDeliveries(cmd, cli, inputs.StreamID)
				if err != nil {
					return fmt.Errorf("failed to retrieve deliveries: %w", err)
				}
				if len(deliveries) == 0 {
					return fmt.Errorf("no deliveries found for stream %q", inputs.StreamID)
				}

				currentIndex := auth0.Int(0)
				selectedDelivery := cli.renderer.EventDeliveryPrompt(deliveries, currentIndex)
				inputs.DeliveryID = selectedDelivery.GetID()
			}

			var delivery *management.EventDelivery
			if err := ansi.Waiting(func() (err error) {
				delivery, err = cli.api.EventStream.ReadDelivery(cmd.Context(), inputs.StreamID, inputs.DeliveryID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to fetch delivery %q for stream %q: %w", inputs.DeliveryID, inputs.StreamID, err)
			}

			cli.renderer.ShowDelivery(delivery)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func redeliverEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		StreamID    string
		DeliveryIDs []string
	}

	cmd := &cobra.Command{
		Use:   "redeliver [stream-id] [comma-separated-delivery-ids]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Retry one or more event deliveries for a given stream",
		Long: "Retry one or more failed event deliveries for a given event stream. \n" +
			"If no delivery IDs are provided, you'll be prompted " +
			"to select from recent failed deliveries.",
		Example: `  auth0 event-streams redeliver
  auth0 event-streams redeliver <stream-id>
  auth0 event-streams redeliver <stream-id> evt_abc123,evt_def456`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				inputs.StreamID = args[0]
			}
			if len(args) == 2 {
				inputs.DeliveryIDs = strings.Split(args[1], ",")
			}

			if inputs.StreamID == "" {
				if err := eventStreamID.Pick(cmd, &inputs.StreamID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			if len(inputs.DeliveryIDs) == 0 {
				selectedIDs, err := promptForDeliveryIDs(cmd, cli, inputs.StreamID)
				if err != nil {
					return err
				}
				inputs.DeliveryIDs = selectedIDs
			}

			for _, id := range inputs.DeliveryIDs {
				err := ansi.Spinner(fmt.Sprintf("Redelivering %s...", ansi.Faint(id)), func() error {
					return cli.api.EventStream.Redeliver(cmd.Context(), inputs.StreamID, id)
				})

				if err != nil {
					cli.renderer.Errorf("%s Failed to redeliver %s: %v\n", ansi.Red("✘"), ansi.Red(id), err)
				}
			}
			return nil
		},
	}

	return cmd
}

func redeliverManyEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		StreamID  string
		EventType string
		From      string
		To        string
	}

	cmd := &cobra.Command{
		Use:   "redeliver-many [stream-id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Bulk retry failed event deliveries using filters",
		Long: "Retry multiple failed event deliveries for a given event stream. \n" +
			"You can filter by event type and date range. \n" +
			"All filters are combined using AND logic. \n" +
			"If no filters are passed, all failed events are retried",
		Example: `  auth0 event-streams redeliver-many
  auth0 event-streams redeliver-many <stream-id>
  auth0 event-streams redeliver-many <stream-id> --type=user.created,user.deleted --from=-2d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				inputs.StreamID = args[0]
			}
			if inputs.StreamID == "" {
				if err := eventStreamID.Pick(cmd, &inputs.StreamID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			var req management.BulkRedeliverRequest

			if inputs.EventType != "" {
				types := strings.Split(inputs.EventType, ",")
				req.EventTypes = &types
			}

			if inputs.From != "" {
				fromDate, err := parseFlexibleDate(inputs.From)
				if err != nil {
					return fmt.Errorf("invalid --from date: %w", err)
				}
				req.DateFrom = &fromDate
			}

			if inputs.To != "" {
				toDate, err := parseFlexibleDate(inputs.To)
				if err != nil {
					return fmt.Errorf("invalid --to date: %w", err)
				}
				req.DateTo = &toDate
			}

			return ansi.Spinner("Bulk redelivering events...", func() error {
				return cli.api.EventStream.RedeliverMany(cmd.Context(), inputs.StreamID, &req)
			})
		},
	}

	cmd.Flags().StringVar(&inputs.EventType, "type", "", "Comma-separated event types (e.g. user.created,user.deleted)")
	cmd.Flags().StringVarP(&inputs.From, "from", "f", "", "Start date for filtering (e.g. 2025-07-25, -2d, yesterday)")
	cmd.Flags().StringVarP(&inputs.To, "to", "t", "", "End date for filtering (e.g. 2025-07-29, today)")

	return cmd
}

func statsEventStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		StreamID string
		From     string
		To       string
	}

	cmd := &cobra.Command{
		Use:   "stats [stream-id]",
		Short: "View delivery stats for an event stream",
		Long: `Retrieve metrics over time for a given event stream, including 
successful and failed delivery counts. Supports custom date range filtering.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  auth0 event-streams stats
  auth0 event-streams stats <stream-id>
  auth0 event-streams stats <stream-id> --from 2025-07-15 --to 2025-07-29`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.StreamID = args[0]
			}

			if inputs.StreamID == "" {
				if err := eventStreamID.Pick(cmd, &inputs.StreamID, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			}

			var opts []management.RequestOption

			if inputs.From != "" {
				fromDate, err := parseFlexibleDate(inputs.From)
				if err != nil {
					return fmt.Errorf("invalid --from date: %w", err)
				}
				opts = append(opts, management.Parameter("date_from", fromDate))
			}

			if inputs.To != "" {
				toDate, err := parseFlexibleDate(inputs.To)
				if err != nil {
					return fmt.Errorf("invalid --to date: %w", err)
				}
				opts = append(opts, management.Parameter("date_to", toDate))
			}

			stats, err := cli.api.EventStream.Stats(cmd.Context(), inputs.StreamID, opts...)
			if err != nil {
				return fmt.Errorf("failed to fetch stats for stream %q: %w", inputs.StreamID, err)
			}
			cli.renderer.RenderEventStreamStats(stats)
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputs.From, "from", "f", "", "Start date for stats (e.g. 2025-07-15, -3d)")
	cmd.Flags().StringVarP(&inputs.To, "to", "t", "", "End date for stats (e.g. 2025-07-29)")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func fetchRecentDeliveries(cmd *cobra.Command, cli *cli, streamID string) ([]*management.EventDelivery, error) {
	var deliveries *management.EventDeliveryList
	err := ansi.Waiting(func() (err error) {
		deliveries, err = cli.api.EventStream.ListDeliveries(cmd.Context(), streamID,
			management.Parameter("take", "100"),
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return deliveries.Deliveries, nil
}

func (c *cli) eventStreamPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.EventStream.List(ctx)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list.EventStreams {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))

		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no event streams to choose from. Create one by running: `auth0 event-streams create`")
	}

	return opts, nil
}

func promptForDeliveryIDs(cmd *cobra.Command, cli *cli, streamID string) ([]string, error) {
	deliveries, err := fetchRecentDeliveries(cmd, cli, streamID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch deliveries: %w", err)
	}
	if len(deliveries) == 0 {
		return nil, fmt.Errorf("no deliveries found for stream %q", streamID)
	}

	labelToID := make(map[string]string)
	var labels []string
	for _, d := range deliveries {
		label := fmt.Sprintf("%s (%s) - %s - %d", d.GetEventType(), d.GetStatus(), d.GetID(), len(d.Attempts))
		labelToID[label] = d.GetID()
		labels = append(labels, label)
	}

	var selectedLabels []string
	if err := prompt.AskMultiSelect("Select deliveries to redeliver", &selectedLabels, labels...); err != nil {
		return nil, err
	}

	var selectedIDs []string
	for _, label := range selectedLabels {
		selectedIDs = append(selectedIDs, labelToID[label])
	}

	return selectedIDs, nil
}
