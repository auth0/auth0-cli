package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	logStreamTypeAmazonEventBridge logStreamType = "eventbridge"
	logStreamTypeAzureEventGrid    logStreamType = "eventgrid"
	logStreamTypeHTTP              logStreamType = "http"
	logStreamTypeDatadog           logStreamType = "datadog"
	logStreamTypeSplunk            logStreamType = "splunk"
	logStreamTypeSumo              logStreamType = "sumo"
)

type logStreamType string

var (
	logStreamID = Argument{
		Name: "Log stream ID",
		Help: "Log stream ID",
	}
	logStreamName = Flag{
		Name:         "Name",
		LongForm:     "name",
		ShortForm:    "n",
		Help:         "The name of the log stream.",
		AlwaysPrompt: true,
	}
)

func logStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "streams",
		Short: "Manage resources for log streams",
		Long: "Auth0's log streaming service allows you to export tenant log events to a log event analysis " +
			"service URL. Log streaming allows you to react to events like password changes or new registrations " +
			"with your own business logic.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listLogStreamsCmd(cli))
	cmd.AddCommand(createLogStreamCmd(cli))
	cmd.AddCommand(showLogStreamCmd(cli))
	cmd.AddCommand(updateLogStreamCmd(cli))
	cmd.AddCommand(deleteLogStreamCmd(cli))
	cmd.AddCommand(openLogStreamsCmd(cli))

	return cmd
}

func listLogStreamsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List all log streams",
		Long:    "List your existing log streams. To create one, run: `auth0 logs streams create`.",
		Example: `  auth0 logs streams list
  auth0 logs streams ls
  auth0 logs streams ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list []*management.LogStream

			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.LogStream.List()
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			cli.renderer.LogStreamList(list)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func showLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Type string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a log stream by ID",
		Long:  "Display information about a log stream.",
		Example: `  auth0 logs streams show
  auth0 logs streams show <log-stream-id>
  auth0 logs streams show <log-stream-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Ask(cmd, &inputs.ID)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			a := &management.LogStream{ID: &inputs.ID}

			if err := ansi.Waiting(func() error {
				var err error
				a, err = cli.api.LogStream.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load log stream: %w", err)
			}
			cli.renderer.LogStreamShow(a)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func createLogStreamCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new log stream",
		Long:  "Log Streaming allows you to export your events in near real-time.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(createLogStreamsAmazonEventBridgeCmd(cli))
	cmd.AddCommand(createLogStreamsAzureEventGridCmd(cli))
	cmd.AddCommand(createLogStreamsCustomWebhookCmd(cli))
	cmd.AddCommand(createLogStreamsDatadogCmd(cli))
	cmd.AddCommand(createLogStreamsSplunkCmd(cli))
	cmd.AddCommand(createLogStreamsSumoLogicCmd(cli))

	return cmd
}

func updateLogStreamCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing log stream",
		Long:  "Log Streaming allows you to export your events in near real-time.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(updateLogStreamsAmazonEventBridgeCmd(cli))
	cmd.AddCommand(updateLogStreamsAzureEventGridCmd(cli))
	cmd.AddCommand(updateLogStreamsCustomWebhookCmd(cli))
	cmd.AddCommand(updateLogStreamsDatadogCmd(cli))
	cmd.AddCommand(updateLogStreamsSplunkCmd(cli))
	cmd.AddCommand(updateLogStreamsSumoLogicCmd(cli))

	return cmd
}

func deleteLogStreamCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete a log stream",
		Long: "Delete a log stream.\n\n" +
			"To delete interactively, use `auth0 logs streams delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the log stream id and the `--force`" +
			" flag to skip confirmation.",
		Example: `  auth0 logs streams delete
  auth0 logs streams rm
  auth0 logs streams delete <log-stream-id>
  auth0 logs streams delete <log-stream-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.allLogStreamsPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting Log Stream", func() error {
				_, err := cli.api.LogStream.Read(inputs.ID)

				if err != nil {
					return fmt.Errorf("Unable to delete log stream: %w", err)
				}

				return cli.api.LogStream.Delete(inputs.ID)
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func openLogStreamsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the settings page of a log stream",
		Long:  "Open a log stream's settings page in the Auth0 Dashboard.",
		Example: `  auth0 logs streams open
  auth0 logs streams open <log-stream-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := logStreamID.Pick(cmd, &inputs.ID, cli.allLogStreamsPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openManageURL(cli, cli.config.DefaultTenant, formatLogStreamSettingsPath(inputs.ID))

			return nil
		},
	}

	return cmd
}

func formatLogStreamSettingsPath(id string) string {
	if len(id) == 0 {
		return ""
	}
	return fmt.Sprintf("log-streams/%s/settings", id)
}

func (c *cli) allLogStreamsPickerOptions() (pickerOptions, error) {
	logStreams, err := c.api.LogStream.List()
	if err != nil {
		return nil, err
	}

	var options pickerOptions
	for _, logStream := range logStreams {
		value := logStream.GetID()
		label := fmt.Sprintf("%s %s", logStream.GetName(), ansi.Faint("("+value+")"))
		options = append(options, pickerOption{value: value, label: label})
	}
	if len(options) == 0 {
		return nil, errors.New("there are currently no log streams")
	}

	return options, nil
}

func (c *cli) logStreamPickerOptionsByType(desiredType logStreamType) pickerOptionsFunc {
	return func() (pickerOptions, error) {
		logStreams, err := c.api.LogStream.List()
		if err != nil {
			return nil, err
		}

		var options pickerOptions
		for _, logStream := range logStreams {
			if logStream.GetType() == string(desiredType) {
				value := logStream.GetID()
				label := fmt.Sprintf("%s %s", logStream.GetName(), ansi.Faint("("+value+")"))
				options = append(options, pickerOption{value: value, label: label})
			}
		}
		if len(options) == 0 {
			return nil, fmt.Errorf("there are currently no log streams of type: %s", desiredType)
		}

		return options, nil
	}
}
