package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	sumoLogicSource = Flag{
		Name:       "Sumo Logic Source",
		LongForm:   "source",
		ShortForm:  "s",
		Help:       "Generated URL for your defined HTTP source in Sumo Logic.",
		IsRequired: true,
	}
)

func createLogStreamsSumoLogicCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name            string
		SumoLogicSource string
	}

	cmd := &cobra.Command{
		Use:   "sumo",
		Args:  cobra.NoArgs,
		Short: "Create a new Sumo Logic log stream",
		Long: "Visualize logs and detect threats faster with security insights.\n\n" +
			"To create interactively, use `auth0 logs streams create sumo` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create sumo
  auth0 logs streams create sumo --name <name>
  auth0 logs streams create sumo --name <name> --source <source>
  auth0 logs streams create sumo -n <name> -s <source>
  auth0 logs streams create sumo -n "mylogstream" -s "demo.sumo.com" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := sumoLogicSource.Ask(cmd, &inputs.SumoLogicSource, nil); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeSumo)),
				Sink: &management.LogStreamSinkSumo{
					SourceAddress: &inputs.SumoLogicSource,
				},
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(cmd.Context(), newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %v", err)
			}

			cli.renderer.LogStreamCreate(newLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.Name, "")
	sumoLogicSource.RegisterString(cmd, &inputs.SumoLogicSource, "")

	return cmd
}

func updateLogStreamsSumoLogicCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		Name            string
		SumoLogicSource string
	}

	cmd := &cobra.Command{
		Use:   "sumo",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Sumo Logic log stream",
		Long: "Visualize logs and detect threats faster with security insights.\n\n" +
			"To update interactively, use `auth0 logs streams create sumo` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams update sumo
  auth0 logs streams update sumo <log-stream-id> --name <name>
  auth0 logs streams update sumo <log-stream-id> --name <name> --source <source>
  auth0 logs streams update sumo <log-stream-id> -n <name> -s <source>
  auth0 logs streams update sumo <log-stream-id> -n "mylogstream" -s "demo.sumo.com" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeSumo))
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var oldLogStream *management.LogStream
			if err := ansi.Waiting(func() (err error) {
				oldLogStream, err = cli.api.LogStream.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with ID %s: %w", inputs.ID, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeSumo) {
				return errInvalidLogStreamType(inputs.ID, oldLogStream.GetType(), string(logStreamTypeSumo))
			}

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			sumoSink := oldLogStream.Sink.(*management.LogStreamSinkSumo)
			if err := sumoLogicSource.AskU(cmd, &inputs.SumoLogicSource, sumoSink.SourceAddress); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}
			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}
			if inputs.SumoLogicSource != "" {
				sumoSink.SourceAddress = &inputs.SumoLogicSource
			}
			updatedLogStream.Sink = sumoSink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(cmd.Context(), oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %s: %w", oldLogStream.GetID(), err)
			}

			cli.renderer.LogStreamUpdate(updatedLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.Name, "")
	sumoLogicSource.RegisterStringU(cmd, &inputs.SumoLogicSource, "")

	return cmd
}
