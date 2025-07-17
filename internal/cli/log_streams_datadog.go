package cli

import (
	"encoding/json"
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
		name          string
		datadogAPIKey string
		datadogRegion string
		piiConfig     string
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
  auth0 logs streams create datadog --name <name> --region <region> --api-key <api-key> --pii-config "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"hash\", \"algorithm\": \"xxhash\"}"
  auth0 logs streams create datadog -n <name> -r <region> -k <api-key>
  auth0 logs streams create datadog -n mylogstream -r eu -k 121233123455 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.name, nil); err != nil {
				return err
			}

			if err := datadogRegion.Select(cmd, &inputs.datadogRegion, datadogRegionOptions, nil); err != nil {
				return err
			}

			if err := datadogAPIKey.AskPassword(cmd, &inputs.datadogAPIKey); err != nil {
				return err
			}

			var piiConfig *management.LogStreamPiiConfig

			if err := logStreamPIIConfig.Ask(cmd, &inputs.piiConfig, auth0.String("{}")); err != nil {
				return err
			}

			if inputs.piiConfig != "{}" {
				if err := json.Unmarshal([]byte(inputs.piiConfig), &piiConfig); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.piiConfig, err)
				}
			}

			newLogStream := &management.LogStream{
				Name: &inputs.name,
				Type: auth0.String(string(logStreamTypeDatadog)),
				Sink: &management.LogStreamSinkDatadog{
					Region: &inputs.datadogRegion,
					APIKey: &inputs.datadogAPIKey,
				},
				PIIConfig: piiConfig,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(cmd.Context(), newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %w", err)
			}

			return cli.renderer.LogStreamCreate(newLogStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.name, "")
	logStreamPIIConfig.RegisterString(cmd, &inputs.piiConfig, "{}")
	datadogAPIKey.RegisterString(cmd, &inputs.datadogAPIKey, "")
	datadogRegion.RegisterString(cmd, &inputs.datadogRegion, "")

	return cmd
}

func updateLogStreamsDatadogCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id            string
		name          string
		piiConfig     string
		datadogAPIKey string
		datadogRegion string
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
  auth0 logs streams update datadog <log-stream-id> --name <name> --region <region> --api-key <api-key> --pii-config "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"mask\", \"algorithm\": \"xxhash\"}"
  auth0 logs streams update datadog <log-stream-id> -n <name> -r <region> -k <api-key> -c null
  auth0 logs streams update datadog <log-stream-id> -n mylogstream -r eu -k 121233123455 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeDatadog))
				if err != nil {
					return err
				}
			} else {
				inputs.id = args[0]
			}

			var oldLogStream *management.LogStream
			if err := ansi.Waiting(func() (err error) {
				oldLogStream, err = cli.api.LogStream.Read(cmd.Context(), inputs.id)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read log stream with ID %q: %w", inputs.id, err)
			}

			if oldLogStream.GetType() != string(logStreamTypeDatadog) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeDatadog))
			}

			if err := logStreamName.AskU(cmd, &inputs.name, oldLogStream.Name); err != nil {
				return err
			}

			existing, _ := json.Marshal(oldLogStream.GetPIIConfig())
			if err := logStreamPIIConfig.AskU(cmd, &inputs.piiConfig, auth0.String(string(existing))); err != nil {
				return err
			}

			datadogSink := oldLogStream.Sink.(*management.LogStreamSinkDatadog)

			if err := datadogRegion.SelectU(cmd, &inputs.datadogRegion, datadogRegionOptions, datadogSink.Region); err != nil {
				return err
			}

			if err := datadogAPIKey.AskPasswordU(cmd, &inputs.datadogAPIKey); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{
				PIIConfig: oldLogStream.GetPIIConfig(),
			}

			if inputs.name != "" {
				updatedLogStream.Name = &inputs.name
			}
			if inputs.datadogRegion != "" {
				datadogSink.Region = &inputs.datadogRegion
			}
			if inputs.datadogAPIKey != "" {
				datadogSink.APIKey = &inputs.datadogAPIKey
			}

			updatedLogStream.Sink = datadogSink

			if inputs.piiConfig != "{}" {
				var piiConfig *management.LogStreamPiiConfig
				if err := json.Unmarshal([]byte(inputs.piiConfig), &piiConfig); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.piiConfig, err)
				}
				updatedLogStream.PIIConfig = piiConfig
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Update(cmd.Context(), oldLogStream.GetID(), updatedLogStream)
			}); err != nil {
				return fmt.Errorf("failed to update log stream with ID %q: %w", oldLogStream.GetID(), err)
			}

			return cli.renderer.LogStreamUpdate(updatedLogStream)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterStringU(cmd, &inputs.name, "")
	logStreamPIIConfig.RegisterStringU(cmd, &inputs.piiConfig, "{}")
	datadogAPIKey.RegisterStringU(cmd, &inputs.datadogAPIKey, "")
	datadogRegion.RegisterStringU(cmd, &inputs.datadogRegion, "")

	return cmd
}
