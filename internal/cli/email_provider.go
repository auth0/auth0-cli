package cli

import (
	"encoding/json"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	emailProviderMandrill  = management.EmailProviderMandrill
	emailProviderSES       = management.EmailProviderSES
	emailProviderSendGrid  = management.EmailProviderSendGrid
	emailProviderSparkPost = management.EmailProviderSparkPost
	emailProviderMailgun   = management.EmailProviderMailgun
	emailProviderSMTP      = management.EmailProviderSMTP
	emailProviderAzureCS   = management.EmailProviderAzureCS
	emailProviderMS365     = management.EmailProviderMS365
	emailProviderCustom    = management.EmailProviderCustom
)

var (
	providerNameOptions = []string{
		emailProviderMandrill,
		emailProviderSES,
		emailProviderSendGrid,
		emailProviderSparkPost,
		emailProviderMailgun,
		emailProviderSMTP,
		emailProviderAzureCS,
		emailProviderMS365,
		emailProviderCustom,
	}

	emailProviderName = Flag{
		Name:      "Provider",
		LongForm:  "provider",
		ShortForm: "p",
		Help: fmt.Sprintf("Provider name. Can be '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', or '%s'",
			emailProviderMandrill,
			emailProviderSES,
			emailProviderSendGrid,
			emailProviderSparkPost,
			emailProviderMailgun,
			emailProviderSMTP,
			emailProviderAzureCS,
			emailProviderMS365,
			emailProviderCustom),
		AlwaysPrompt: true,
	}

	emailProviderFrom = Flag{
		Name:         "DefaultFromAddress",
		LongForm:     "default-from-address",
		ShortForm:    "f",
		Help:         "Provider default FROM address if none is specified.",
		AlwaysPrompt: true,
	}

	emailProviderCredentials = Flag{
		Name:         "Credentials",
		LongForm:     "credentials",
		ShortForm:    "c",
		Help:         "Credentials for the email provider, formatted as JSON.",
		AlwaysPrompt: true,
	}

	emailProviderSettings = Flag{
		Name:         "Settings",
		LongForm:     "settings",
		ShortForm:    "s",
		Help:         "Settings for the email provider. formatted as JSON.",
		AlwaysPrompt: true,
	}

	emailProviderEnabled = Flag{
		Name:         "Enabled",
		LongForm:     "enabled",
		ShortForm:    "e",
		Help:         "Whether the provided is enabled (true) or disabled (false).",
		AlwaysPrompt: true,
	}
)

func emailProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage custom email provider",
		Long:  "Manage custom email provider for the tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showEmailProviderCmd(cli))
	cmd.AddCommand(createEmailProviderCmd(cli))
	cmd.AddCommand(updateEmailProviderCmd(cli))
	cmd.AddCommand(deleteEmailProviderCmd(cli))
	return cmd
}

func showEmailProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show the email provider",
		Long:  "Display information about the email provider.",
		Example: `  auth0 email provider show
  auth0 email provider show --json
  auth0 email provider show --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var emailProvider *management.EmailProvider
			if err := ansi.Waiting(func() (err error) {
				emailProvider, err = cli.api.EmailProvider.Read(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read email provider: %w", err)
			}

			return cli.renderer.EmailProviderShow(emailProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createEmailProviderCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name               string
		defaultFromAddress string
		credentials        string
		settings           string
		enabled            bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create the email provider",
		Long: "Create the email provider.\n\n" +
			"To create interactively, use `auth0 email provider create` with no arguments.\n\n" +
			"To create non-interactively, supply the provider name and other information " +
			"through the flags.",
		Example: `  auth0 email provider create
  auth0 email provider create --json
  auth0 email provider create --json-compact
  auth0 email provider create --provider mandrill --enabled=true --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider create --provider mandrill --default-from-address='admin@example.com' --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider create --provider ses --credentials='{ "accessKeyId":"TheAccessKeyId", "secretAccessKey":"TheSecretAccessKey", "region":"eu" }' --settings='{ "message": { "configuration_set_name": "TheConfigurationSetName" } }'
  auth0 email provider create --provider sendgrid --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider create --provider sparkpost --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider create --provider sparkpost --credentials='{ "api_key":"TheAPIKey", "region":"eu" }'
  auth0 email provider create --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com"}'
  auth0 email provider create --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com", "region":"eu" }'
  auth0 email provider create --provider smtp --credentials='{ "smtp_host":"smtp.example.com", "smtp_port":25, "smtp_user":"smtp", "smtp_pass":"TheSMTPPassword" }'
  auth0 email provider create --provider azure_cs --credentials='{ "connection_string":"TheConnectionString" }'
  auth0 email provider create --provider ms365 --credentials='{ "tenantId":"TheTenantId", "clientId":"TheClientID", "clientSecret":"TheClientSecret" }'
  auth0 email provider create --provider custom --enabled=true --default-from-address="admin@example.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := emailProviderName.Select(cmd, &inputs.name, providerNameOptions, nil); err != nil {
				return err
			}
			if err := emailProviderFrom.Ask(cmd, &inputs.defaultFromAddress, nil); err != nil {
				return err
			}
			if err := emailProviderEnabled.AskBool(cmd, &inputs.enabled, nil); err != nil {
				return err
			}

			var credentials map[string]interface{}
			if inputs.name == emailProviderCustom {
				if len(inputs.credentials) > 0 {
					return fmt.Errorf("credentials not supported for provider: %s", inputs.name)
				}
				credentials = make(map[string]interface{})
			} else {
				if err := emailProviderCredentials.Ask(cmd, &inputs.credentials, nil); err != nil {
					return err
				}
				if err := json.Unmarshal([]byte(inputs.credentials), &credentials); err != nil {
					return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.name, err)
				}
			}

			var settings map[string]interface{}
			switch inputs.name {
			case emailProviderMandrill, emailProviderSES, emailProviderSMTP:
				if err := emailProviderSettings.Ask(cmd, &inputs.settings, nil); err != nil {
					return err
				}
				if len(inputs.settings) > 0 {
					if err := json.Unmarshal([]byte(inputs.settings), &settings); err != nil {
						return fmt.Errorf("provider: %s settings invalid JSON: %w", inputs.name, err)
					}
				}
			case emailProviderSendGrid,
				emailProviderSparkPost,
				emailProviderMailgun,
				emailProviderAzureCS,
				emailProviderMS365,
				emailProviderCustom:
				if len(inputs.settings) > 0 {
					return fmt.Errorf("settings not supported for provider: %s", inputs.name)
				}
			default:
				return fmt.Errorf("unknown provider: %s", inputs.name)
			}

			emailProvider := &management.EmailProvider{
				Name:    &inputs.name,
				Enabled: &inputs.enabled,
			}

			if len(inputs.defaultFromAddress) > 0 {
				emailProvider.DefaultFromAddress = &inputs.defaultFromAddress
			}

			if credentials != nil {
				emailProvider.Credentials = &credentials
			}

			if settings != nil {
				emailProvider.Settings = &settings
			}

			if err := ansi.Waiting(func() error {
				return cli.api.EmailProvider.Create(cmd.Context(), emailProvider)
			}); err != nil {
				return fmt.Errorf("failed to create email provider %s: %w", inputs.name, err)
			}

			return cli.renderer.EmailProviderCreate(emailProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	emailProviderName.RegisterString(cmd, &inputs.name, "")
	emailProviderFrom.RegisterString(cmd, &inputs.defaultFromAddress, "")
	emailProviderCredentials.RegisterString(cmd, &inputs.credentials, "")
	emailProviderSettings.RegisterString(cmd, &inputs.settings, "")
	emailProviderEnabled.RegisterBool(cmd, &inputs.enabled, true)

	return cmd
}

func updateEmailProviderCmd(cli *cli) *cobra.Command {
	var inputs struct {
		name               string
		defaultFromAddress string
		credentials        string
		settings           string
		enabled            bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update the email provider",
		Long: "Update the email provider.\n\n" +
			"To update interactively, use `auth0 email provider update` with no arguments.\n\n" +
			"To update non-interactively, supply the provider name and other information " +
			"through the flags.",
		Example: `  auth0 email provider update
  auth0 email provider update --json
  auth0 email provider update --json-compact
  auth0 email provider update --enabled=false
  auth0 email provider update --credentials='{ "api_key":"NewAPIKey" }'
  auth0 email provider update --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --default-from-address="admin@example.com"
  auth0 email provider update --provider mandrill --enabled=true --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --provider mandrill --default-from-address='admin@example.com' --credentials='{ "api_key":"TheAPIKey" }' --settings='{ "message": { "view_control_link": true } }'
  auth0 email provider update --provider ses --credentials='{ "accessKeyId":"TheAccessKeyId", "secretAccessKey":"TheSecretAccessKey", "region":"eu" }' --settings='{ "message": { "configuration_set_name": "TheConfigurationSetName" } }'
  auth0 email provider update --provider sendgrid --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider update --provider sparkpost --credentials='{ "api_key":"TheAPIKey" }'
  auth0 email provider update --provider sparkpost --credentials='{ "api_key":"TheAPIKey", "region":"eu" }'
  auth0 email provider update --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com"}'
  auth0 email provider update --provider mailgun --credentials='{ "api_key":"TheAPIKey", "domain": "example.com", "region":"eu" }'
  auth0 email provider update --provider smtp --credentials='{ "smtp_host":"smtp.example.com", "smtp_port":25, "smtp_user":"smtp", "smtp_pass":"TheSMTPPassword" }'
  auth0 email provider update --provider azure_cs --credentials='{ "connection_string":"TheConnectionString" }'
  auth0 email provider update --provider ms365 --credentials='{ "tenantId":"TheTenantId", "clientId":"TheClientID", "clientSecret":"TheClientSecret" }'
  auth0 email provider update --provider custom --enabled=true --default-from-address="admin@example.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var currentProvider *management.EmailProvider

			if err := ansi.Waiting(func() (err error) {
				currentProvider, err = cli.api.EmailProvider.Read(cmd.Context())
				return
			}); err != nil {
				return fmt.Errorf("failed to read email provider: %w", err)
			}

			if err := emailProviderName.SelectU(cmd, &inputs.name, providerNameOptions, currentProvider.Name); err != nil {
				return err
			}
			if err := emailProviderFrom.AskU(cmd, &inputs.defaultFromAddress, currentProvider.DefaultFromAddress); err != nil {
				return err
			}
			if err := emailProviderEnabled.AskBoolU(cmd, &inputs.enabled, currentProvider.Enabled); err != nil {
				return err
			}

			var credentials map[string]interface{}
			var settings map[string]interface{}

			emailProvider := &management.EmailProvider{}

			// Check if we are changing providers.
			if len(inputs.name) > 0 && inputs.name != currentProvider.GetName() {
				// Only set the name if we are changing it.
				emailProvider.Name = &inputs.name

				// If we are changing providers, we need new credentials and settings.
				if inputs.name == emailProviderCustom {
					if len(inputs.credentials) > 0 {
						return fmt.Errorf("credentials not supported for provider: %s", inputs.name)
					}
					credentials = make(map[string]interface{})
				} else {
					if err := emailProviderCredentials.AskU(cmd, &inputs.credentials, nil); err != nil {
						return err
					}
					if err := json.Unmarshal([]byte(inputs.credentials), &credentials); err != nil {
						return fmt.Errorf("provider: %s credentials invalid JSON: %w", inputs.name, err)
					}
				}

				switch inputs.name {
				case emailProviderMandrill, emailProviderSES, emailProviderSMTP:
					if err := emailProviderSettings.AskU(cmd, &inputs.settings, nil); err != nil {
						return err
					}
					if len(inputs.settings) > 0 {
						if err := json.Unmarshal([]byte(inputs.settings), &settings); err != nil {
							return fmt.Errorf("provider: %s settings invalid JSON: %w", inputs.name, err)
						}
					}
				case emailProviderSendGrid,
					emailProviderSparkPost,
					emailProviderMailgun,
					emailProviderAzureCS,
					emailProviderMS365,
					emailProviderCustom:
					if len(inputs.settings) > 0 {
						return fmt.Errorf("settings not supported for provider: %s", inputs.name)
					}
				default:
					return fmt.Errorf("unknown provider: %s", inputs.name)
				}
			}

			// Set the flag if it was supplied or entered by the prompt.
			if emailProviderEnabled.IsSet(cmd) || noLocalFlagSet(cmd) {
				emailProvider.Enabled = &inputs.enabled
			}

			if len(inputs.defaultFromAddress) > 0 {
				emailProvider.DefaultFromAddress = &inputs.defaultFromAddress
			}

			if credentials != nil {
				emailProvider.Credentials = &credentials
			}

			if settings != nil {
				emailProvider.Settings = &settings
			}

			if err := ansi.Waiting(func() error {
				return cli.api.EmailProvider.Update(cmd.Context(), emailProvider)
			}); err != nil {
				return fmt.Errorf("failed to update email provider %s: %w", inputs.name, err)
			}

			return cli.renderer.EmailProviderUpdate(emailProvider)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	emailProviderName.RegisterString(cmd, &inputs.name, "")
	emailProviderFrom.RegisterString(cmd, &inputs.defaultFromAddress, "")
	emailProviderCredentials.RegisterString(cmd, &inputs.credentials, "")
	emailProviderSettings.RegisterString(cmd, &inputs.settings, "")
	emailProviderEnabled.RegisterBool(cmd, &inputs.enabled, true)

	return cmd
}

func deleteEmailProviderCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.NoArgs,
		Short:   "Delete the email provider",
		Long: "Delete the email provider.\n\n" +
			"To delete interactively, use `auth0 email provider delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the the `--force`" +
			" flag to skip confirmation.",
		Example: `  auth0 provider delete
  auth0 email provider rm
  auth0 email provider delete --force
  auth0 email provider rm --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.api.EmailProvider.Delete(cmd.Context())
			}); err != nil {
				return fmt.Errorf("failed to delete email provider: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}
