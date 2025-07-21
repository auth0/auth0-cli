package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

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
)

func eventStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
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

	return cmd
}

func listEventStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your event streams",
		Long:    "List your existing event streams. To create one, run: `auth0 events create`.",
		Example: `  auth0 events list
  auth0 events ls
  auth0 events ls --json
  auth0 events ls --csv`,
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
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "csv")

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
		Example: `  auth0 events show
  auth0 events show <event-id>
  auth0 events show <event-id> --json`,
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
			"To create interactively, use `auth0 events create` with no flags.\n\n" +
			"To create non-interactively, supply the event stream name, type, subscriptions and configuration through the flags.",
		Example: `  auth0 events create
  auth0 events create --name my-event-stream --type eventbridge --subscriptions "user.created,user.updated" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 events create --name my-event-stream --type webhook --subscriptions "user.created,user.deleted" --configuration '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
  auth0 events create -n my-event-stream -t webhook -s "user.created,user.deleted" -c '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'`,
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
		Short: "Update an event",
		Long: "Update an event.\n\n" +
			"To update interactively, use `auth0 events update` with no arguments.\n\n" +
			"To update non-interactively, supply the event id, name, status, subscriptions and " +
			"configuration through the flags. An event stream type CANNOT be updated hence the configuration " +
			"should match the schema based on the type of event stream",
		Example: `  auth0 events update <event-id>
  auth0 events update <event-id> --name my-event-stream
  auth0 events update <event-id> --name my-event-stream --status enabled
  auth0 events update <event-id> --name my-event-stream --status enabled --subscriptions "user.created,user.updated"
  auth0 events update <event-id> --name my-event-stream --status disabled --subscriptions "user.deleted" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 events update <event-id> --name my-event-stream --status enabled --subscriptions "user.created" --configuration '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"0909090909"}}
  auth0 events update <event-id> -n my-event-stream --status enabled -s "user.created" -c '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"987654321"}}`,
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

			if err := eventStreamName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if err := eventStreamStatus.AskU(cmd, &inputs.Status, nil); err != nil {
				return err
			}

			if err := eventStreamSubscriptions.AskManyU(cmd, &inputs.Subscriptions, nil); err != nil {
				return err
			}

			if err := eventStreamConfig.AskU(cmd, &inputs.Configuration, nil); err != nil {
				return err
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
			"To delete interactively, use `auth0 events delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the event id and the `--force` flag to skip confirmation.",
		Example: `  auth0 events delete
  auth0 events rm
  auth0 events delete <event-id>
  auth0 events delete <event-id> --force
  auth0 events delete <event-id> <event-id2> <event-idn>
  auth0 events delete <event-id> <event-id2> <event-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, len(args))
			if len(args) == 0 {
				if err := eventStreamID.PickMany(cmd, &ids, cli.eventStreamPickerOptions); err != nil {
					return err
				}
			} else {
				ids = append(ids, args...)
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
		return nil, errors.New("there are currently no event streams to choose from. Create one by running: `auth0 events create`")
	}

	return opts, nil
}
