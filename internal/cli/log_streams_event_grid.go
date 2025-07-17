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
		name                string
		azureSubscriptionID string
		azureRegion         string
		azureResourceGroup  string
		piiConfig           string
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
  auth0 logs streams create eventgrid --name <name> --azure-id <azure-id> --azure-region <azure-region> --azure-group <azure-group> --pii-config  "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"hash\", \"algorithm\": \"xxhash\"}"
  auth0 logs streams create eventgrid -n <name> -i <azure-id> -r <azure-region> -g <azure-group>
  auth0 logs streams create eventgrid -n mylogstream -i "b69a6835-57c7-4d53-b0d5-1c6ae580b6d5" -r northeurope -g "azure-logs-rg" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logStreamName.Ask(cmd, &inputs.name, nil); err != nil {
				return err
			}

			if err := azureSubscriptionID.Ask(cmd, &inputs.azureSubscriptionID, nil); err != nil {
				return err
			}

			if err := azureRegion.Ask(cmd, &inputs.azureRegion, nil); err != nil {
				return err
			}

			if err := azureResourceGroup.Ask(cmd, &inputs.azureResourceGroup, nil); err != nil {
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
				Type: auth0.String(string(logStreamTypeAzureEventGrid)),
				Sink: &management.LogStreamSinkAzureEventGrid{
					SubscriptionID: &inputs.azureSubscriptionID,
					ResourceGroup:  &inputs.azureResourceGroup,
					Region:         &inputs.azureRegion,
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
	azureSubscriptionID.RegisterString(cmd, &inputs.azureSubscriptionID, "")
	azureRegion.RegisterString(cmd, &inputs.azureRegion, "")
	azureResourceGroup.RegisterString(cmd, &inputs.azureResourceGroup, "")

	return cmd
}

func updateLogStreamsAzureEventGridCmd(cli *cli) *cobra.Command {
	var inputs struct {
		id        string
		name      string
		piiConfig string
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
  auth0 logs streams update eventgrid <log-stream-id> -n <name> --pii-config  "{\"log_fields\": [\"first_name\", \"last_name\"], \"method\": \"mask\", \"algorithm\": \"xxhash\"}"
  auth0 logs streams update eventgrid <log-stream-id> -n mylogstream -c null --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				err := logStreamID.Pick(cmd, &inputs.id, cli.logStreamPickerOptionsByType(logStreamTypeAzureEventGrid))
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

			if oldLogStream.GetType() != string(logStreamTypeAzureEventGrid) {
				return errInvalidLogStreamType(inputs.id, oldLogStream.GetType(), string(logStreamTypeAzureEventGrid))
			}

			if err := logStreamName.AskU(cmd, &inputs.name, oldLogStream.Name); err != nil {
				return err
			}

			updatedLogStream := &management.LogStream{}
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

			existing, _ := json.Marshal(oldLogStream.GetPIIConfig())
			if err := logStreamPIIConfig.AskU(cmd, &inputs.piiConfig, auth0.String(string(existing))); err != nil {
				return err
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
