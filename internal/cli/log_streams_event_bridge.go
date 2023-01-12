package cli

import (
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
		Name         string
		AwsAccountID string
		AwsRegion    string
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
  auth0 logs streams create eventbridge -n <name> -i <aws-id> -r <aws-region>
  auth0 logs streams create eventbridge -n mylogstream -i 999999999999 -r "eu-west-1" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := awsAccountID.Ask(cmd, &inputs.AwsAccountID, nil); err != nil {
				return err
			}

			if err := awsRegion.Ask(cmd, &inputs.AwsRegion, nil); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeAmazonEventBridge)),
				Sink: &management.LogStreamSinkAmazonEventBridge{
					AccountID: &inputs.AwsAccountID,
					Region:    &inputs.AwsRegion,
				},
			}

			if err := ansi.Waiting(func() error {
				return cli.api.LogStream.Create(newLogStream)
			}); err != nil {
				return fmt.Errorf("failed to create log stream: %w", err)
			}

			cli.renderer.LogStreamCreate(newLogStream)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	logStreamName.RegisterString(cmd, &inputs.Name, "")
	awsAccountID.RegisterString(cmd, &inputs.AwsAccountID, "")
	awsRegion.RegisterString(cmd, &inputs.AwsRegion, "")

	return cmd
}

func updateLogStreamsAmazonEventBridgeCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Name string
	}

	cmd := &cobra.Command{
		Use:   "eventbridge",
		Args:  cobra.NoArgs,
		Short: "Update an existing Amazon Event Bridge log stream",
		Long: "Stream real-time Auth0 data to over 15 targets like AWS Lambda.\n\n" +
			"To update interactively, use `auth0 logs streams create eventbridge` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name through the flag.",
		Example: `  auth0 logs streams update eventbridge
  auth0 logs streams update eventbridge <log-stream-id> --name <name>
  auth0 logs streams update eventbridge <log-stream-id> -n <name>
  auth0 logs streams update eventbridge <log-stream-id> -n mylogstream --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeAmazonEventBridge))
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

			if err := logStreamName.AskU(cmd, &inputs.Name, oldLogStream.Name); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}

			if inputs.Name != "" {
				updatedLogStream.Name = &inputs.Name
			}

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

	return cmd
}
