package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	azureSubscriptionID = Flag{
		Name:       "Azure Subscription ID",
		LongForm:   "azure-id",
		ShortForm:  "i",
		Help:       "Id of the Azure subscription.",
		IsRequired: true,
	}
	azureRegion = Flag{
		Name:       "Azure Region",
		LongForm:   "azure-region",
		ShortForm:  "r",
		Help:       "The region in which the Azure subscription is hosted.",
		IsRequired: true,
	}
	azureResourceGroup = Flag{
		Name:       "Azure Resource Group",
		LongForm:   "azure-group",
		ShortForm:  "g",
		Help:       "The name of the Azure resource group.",
		IsRequired: true,
	}
)

func createLogStreamsAzureEventGridCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name                string
		AzureSubscriptionID string
		AzureRegion         string
		AzureResourceGroup  string
	}

	cmd := &cobra.Command{
		Use:   "eventgrid",
		Args:  cobra.NoArgs,
		Short: "Create a new Azure Event Grid log stream",
		Long: "A single service for routing events from any source to destination.\n\n" +
			"To create interactively, use `auth0 logs streams create eventgrid` with no arguments.\n\n" +
			"To create non-interactively, supply the log stream name and other information through the flags.",
		Example: `  auth0 logs streams create eventgrid
  auth0 logs streams create eventgrid --name <name>
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> 
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region>
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region> --azure-group <azure-group>
  auth0 logs streams create eventgrid -n <name> -i <azure-id> -r <azure-region> -g <azure-group>
  auth0 logs streams create eventgrid -n mylogstream -i "b69a6835-57c7-4d53-b0d5-1c6ae580b6d5" -r northeurope -g "azure-logs-rg" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := azureSubscriptionID.Ask(cmd, &inputs.AzureSubscriptionID, nil); err != nil {
				return err
			}

			if err := azureRegion.Ask(cmd, &inputs.AzureRegion, nil); err != nil {
				return err
			}

			if err := azureResourceGroup.Ask(cmd, &inputs.AzureResourceGroup, nil); err != nil {
				return err
			}

			newLogStream := &management.LogStream{
				Name: &inputs.Name,
				Type: auth0.String(string(logStreamTypeAzureEventGrid)),
				Sink: &management.LogStreamSinkAzureEventGrid{
					SubscriptionID: &inputs.AzureSubscriptionID,
					ResourceGroup:  &inputs.AzureResourceGroup,
					Region:         &inputs.AzureRegion,
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
	azureSubscriptionID.RegisterString(cmd, &inputs.AzureSubscriptionID, "")
	azureRegion.RegisterString(cmd, &inputs.AzureRegion, "")
	azureResourceGroup.RegisterString(cmd, &inputs.AzureResourceGroup, "")

	return cmd
}

func updateLogStreamsAzureEventGridCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Name string
	}

	cmd := &cobra.Command{
		Use:   "eventgrid",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an existing Azure Event Grid log stream",
		Long: "A single service for routing events from any source to destination.\n\n" +
			"To update interactively, use `auth0 logs streams create eventgrid` with no arguments.\n\n" +
			"To update non-interactively, supply the log stream name through the flag.",
		Example: `  auth0 logs streams update eventgrid
  auth0 logs streams update eventgrid <log-stream-id> --name <name>
  auth0 logs streams update eventgrid <log-stream-id> -n <name>
  auth0 logs streams update eventgrid <log-stream-id> -n mylogstream --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.ID, cli.logStreamPickerOptionsByType(logStreamTypeAzureEventGrid))
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

			if oldLogStream.GetType() != string(logStreamTypeAzureEventGrid) {
				return errInvalidLogStreamType(inputs.ID, oldLogStream.GetType(), string(logStreamTypeAzureEventGrid))
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
