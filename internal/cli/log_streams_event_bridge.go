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
	awsAccountID = Flag{
		Name:       "AWS Account ID",
		LongForm:   "aws-id",
		ShortForm:  "i",
		Help:       "ID of the AWS account.",
		IsRequired: true,
	}
	awsRegion = Flag{
		Name:       "AWS Region",
		LongForm:   "aws-region",
		ShortForm:  "r",
		Help:       "The AWS region in which eventbridge will be created, e.g. 'us-east-2'.",
		IsRequired: true,
	}
)

func createLogStreamsAmazonEventBridgeCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name         string
		awsAccountID string
		awsRegion    string
		piiConfig    string
	}

	cmd := &cobra.Command{
		Use:   "eventbridge",
		Args:  cobra.NoArgs,
		Short: "Create a new Amazon Event Bridge log stream",
		Long: "Stream real-time Auth0 data to over 15 targets like AWS Lambda.\n\n" +
			"To create interactively, use `auth0 logs streams create eventbridge` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create eventbridge
  auth0 logs streams create eventbridge --name <name>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id> --aws-region <aws-region>
  auth0 logs streams create eventbridge --name <name> --aws-id <aws-id> --aws-region <aws-region> --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams create eventbridge -n <name> -i <aws-id> -r <aws-region>
  auth0 logs streams create eventbridge -n mylogstream -i 999999999999 -r "eu-west-1" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.name, nil); err != nil {
				return err
			}

			if err := awsAccountID.Ask(cmd, &inputs.awsAccountID, nil); err != nil {
				return err
			}

			if err := awsRegion.Ask(cmd, &inputs.awsRegion, nil); err != nil {
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
				Type: auth0.String(string(logStreamTypeAmazonEventBridge)),
				Sink: &management.LogStreamSinkAmazonEventBridge{
					AccountID: &inputs.awsAccountID,
					Region:    &inputs.awsRegion,
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
	awsAccountID.RegisterString(cmd, &inputs.awsAccountID, "")
	awsRegion.RegisterString(cmd, &inputs.awsRegion, "")

	return cmd
}

func updateLogStreamsAmazonEventBridgeCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id        string
		name      string
		piiConfig string
	}

	cmd := &cobra.Command{
		Use:   "eventbridge",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Amazon Event Bridge log stream",
		Long: "Stream real-time Auth0 data to over 15 targets like AWS Lambda.\n\n" +
			"To update interactively, use `auth0 logs streams create eventbridge` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name through the flag.",
		Example: `  auth0 logs streams update eventbridge
  auth0 logs streams update eventbridge <log-stream-id> --name <name>
  auth0 logs streams update eventbridge <log-stream-id> --name <name>  --pii-config '{"log_fields": ["first_name", "last_name"], "method": "mask", "algorithm": "xxhash"}'
  auth0 logs streams update eventbridge <log-stream-id> -n <name> -p null
  auth0 logs streams update eventbridge <log-stream-id> -n mylogstream --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeAmazonEventBridge))
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

			if oldLogStream.GetType() != string(logStreamTypeAmazonEventBridge) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeAmazonEventBridge))
			}

			if err := logStreamName.AskU(cmd, &inputs.name, oldLogStream.Name); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{
				PIIConfig: oldLogStream.GetPIIConfig(),
			}

			if inputs.name != "" {
				updatedLogStream.Name = &inputs.name
			}

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

	return cmd
}
