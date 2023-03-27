package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	datadogRegion = Flag{
		Name:      "Datadog Region",
		LongForm:  "region",
		ShortForm: "r",
		Help: "The region in which the datadog dashboard is created.\n" +
			"If you are in the datadog EU site ('app.datadoghq.eu'), the Region should be EU otherwise it should be US.",
		IsRequired: true,
	}

	datadogRegionOptions = []string{"eu", "us", "us3", "us5"}

	datadogAPIKey = Flag{
		Name:       "Datadog API Key",
		LongForm:   "api-key",
		ShortForm:  "k",
		Help:       "Datadog API Key. To obtain a key, see the Datadog Authentication documentation (https://docs.datadoghq.com/api/latest/authentication).",
		IsRequired: true,
	}
)

func createLogStreamsDatadogCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name          string
		DatadogAPIKey string
		DatadogRegion string
	}

	cmd := &cobra.Command{
		Use:   "datadog",
		Args:  cobra.NoArgs,
		Short: "Create a new Datadog log stream",
		Long: "Build interactive dashboards and get alerted on critical issues.\n\n" +
			"To create interactively, use `auth0 logs streams create datadog` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create datadog
  auth0 logs streams create datadog --name <name>
  auth0 logs streams create datadog --name <name> --region <region>
  auth0 logs streams create datadog --name <name> --region <region> --api-key <api-key>
  auth0 logs streams create datadog -n <name> -r <region> -k <api-key>
  auth0 logs streams create datadog -n mylogstream -r eu -k 121233123455 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := datadogRegion.Select(cmd, &inputs.DatadogRegion, datadogRegionOptions, nil); err != nil {
				return err
			}

			if err := datadogAPIKey.AskPassword(cmd, &inputs.DatadogAPIKey); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeDatadog)),
				Sink: &management.LogStreamSinkDatadog{
					Region: &inputs.DatadogRegion,
					APIKey: &inputs.DatadogAPIKey,
				},
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %v", err)
			}

			cli.renderer.LogStreamCreate(newLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.Name, "")
	datadogAPIKey.RegisterString(cmd, &inputs.DatadogAPIKey, "")
	datadogRegion.RegisterString(cmd, &inputs.DatadogRegion, "")

	return cmd
}

func updateLogStreamsDatadogCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID            string
		Name          string
		DatadogAPIKey string
		DatadogRegion string
	}

	cmd := &cobra.Command{
		Use:   "datadog",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Datadog log stream",
		Long: "Build interactive dashboards and get alerted on critical issues.\n\n" +
			"To update interactively, use `auth0 logs streams create datadog` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams update datadog
  auth0 logs streams update datadog <log-stream-id> --name <name>
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region>
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region> --api-key <api-key>
  auth0 logs streams update datadog <log-stream-id> -n <name> -r <region> -k <api-key>
  auth0 logs streams update datadog <log-stream-id> -n mylogstream -r eu -k 121233123455 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeDatadog))
				if err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var oldLogStream *management.LogStream
			if err := ansi.Waiting(func() (err error) {
				oldLogStream, err = cli.api.LogStream.Read(inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with ID %s: %w", inputs.ID, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeDatadog) {
				return errInvalidLogStreamType(inputs.ID, oldLogStream.GetType(), string(logStreamTypeDatadog))
			}

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			datadogSink := oldLogStream.Sink.(*management.LogStreamSinkDatadog)

			if err := datadogRegion.SelectU(cmd, &inputs.DatadogRegion, datadogRegionOptions, datadogSink.Region); err != nil {
				return err
			}

			if err := datadogAPIKey.AskPasswordU(cmd, &inputs.DatadogAPIKey); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}

			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}
			if inputs.DatadogRegion != "" {
				datadogSink.Region = &inputs.DatadogRegion
			}
			if inputs.DatadogAPIKey != "" {
				datadogSink.APIKey = &inputs.DatadogAPIKey
			}

			updatedLogStream.Sink = datadogSink

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %s: %w", oldLogStream.GetID(), err)
			}

			cli.renderer.LogStreamUpdate(updatedLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.Name, "")
	datadogAPIKey.RegisterStringU(cmd, &inputs.DatadogAPIKey, "")
	datadogRegion.RegisterStringU(cmd, &inputs.DatadogRegion, "")

	return cmd
}
