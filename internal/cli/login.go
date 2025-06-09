package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/keyring"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	loginTenantDomain = Flag{
		Name:         "Tenant Domain",
		LongForm:     "domain",
		Help:         "Tenant domain of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginClientID = Flag{
		Name:         "Client ID",
		LongForm:     "client-id",
		Help:         "Client ID of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginClientSecret = Flag{
		Name:         "Client Secret",
		LongForm:     "client-secret",
		Help:         "Client secret of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginAdditionalScopes = Flag{
		Name:         "Additional Scopes",
		LongForm:     "scopes",
		Help:         "Additional scopes to request when authenticating via device code flow. By default, only scopes for first-class functions are requested. Primarily useful when using the api command to execute arbitrary Management API requests.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginTenantProfileLabel = Flag{
		Name:         "Tenant Profile Label",
		LongForm:     "profile",
		Help:         "Tenant Profile Label name to load Auth0 credentials from. If not provided, the default profile will be used.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}
)

type LoginInputs struct {
	Domain           string
	ClientID         string
	ClientSecret     string
	AdditionalScopes []string
	TenantProfile    string
}

func (i *LoginInputs) isLoggingInWithAdditionalScopes() bool {
	return len(i.AdditionalScopes) > 0
}

func loadAuth0Credentials(profile string, inputs *LoginInputs) error {
	credPath := os.Getenv("AUTH0_CREDENTIALS_FILE")
	if credPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		credPath = filepath.Join(home, ".auth0", "credentials")
	}

	cfg, err := ini.Load(credPath)
	if err != nil {
		return err
	}

	sec := cfg.Section(profile)
	if sec == nil {
		return fmt.Errorf("profile %q not found", profile)
	}

	inputs.Domain = sec.Key("domain").String()
	inputs.ClientID = sec.Key("client_id").String()
	inputs.ClientSecret = sec.Key("client_secret").String()

	return nil
}

func loginCmd(cli *cli) *cobra.Command {
	var (
		inputs LoginInputs
	)

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long: "Authenticates the Auth0 CLI using either personal credentials (user login) or client credentials (machine login)." +
			"\n\nUse user login on personal machines or interactive environments (not supported for Private Cloud users).\n" +
			"Use machine login for servers, CI, or any non-interactive environments — " +
			"this is the recommended method for Private Cloud users.\n\n",
		Example: `  auth0 login
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
  auth0 login --scopes "read:client_grants,create:client_grants"
  auth0 login --profile <tenant-profile>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.TenantProfile != "" {
				err := loadAuth0Credentials(inputs.TenantProfile, &inputs)
				if err != nil {
					return fmt.Errorf("failed to load auth0 credentials from %q: %v", inputs.TenantProfile, err)
				}
			}

			var selectedLoginType string
			const loginAsUser, loginAsMachine = "As a user", "As a machine"
			shouldLoginAsUser, shouldLoginAsMachine := false, false

			/*
				Based on the initial inputs we'd like to determine if
				it's a machine login or a user login
				If we successfully determine it, we don't need to prompt the user.

				The --no-input flag add strict restriction that we shall not take any further input after
				initial command.
				Hence, the flow diverges into two based on no-input flag's value.
			*/
			switch {
			case cli.noInput:
				switch {
				case inputs.Domain != "" && inputs.ClientSecret != "" && inputs.ClientID != "":
					// If all three fields are passed, machine login flag is set to true.
					shouldLoginAsMachine = true
				case inputs.Domain != "" && inputs.ClientSecret == "" && inputs.ClientID == "":
					/*
						The domain flag is common between Machine and User Login.
						If domain is passed without client-id and client-secret,
						it can be evaluated that it is a user login flow.
					*/
					shouldLoginAsUser = true
				case inputs.Domain != "" || inputs.ClientSecret != "" || inputs.ClientID != "":
					/*
						At this point, if AT LEAST one of the three flags are passed but not ALL three,
						we return an error since it's a no-input flow and it will need all three params
						for successful machine flow.
						Note that we already determined it's not a user login flow in the condition above.
					*/
					return fmt.Errorf("flags client-id, client-secret and domain are required together")
				default:
					/*
						If no flags are passed along with --no-input, it is defaulted to user login flow.
					*/
					shouldLoginAsUser = true
				}
			default:
				if inputs.ClientSecret != "" || inputs.ClientID != "" {
					/*
						If all three params are passed, we evaluate it as a Machine Login Flow.
						Else required params are prompted for.
					*/
					shouldLoginAsMachine = true
				}
			}

			// If additional scopes are passed we mark shouldLoginAsUser flag to be true.
			if inputs.isLoggingInWithAdditionalScopes() {
				shouldLoginAsUser = true
			}

			/*
				If we are unable to determine if it's a user login or a machine login
				based on all the evaluation above, we go on to prompt the user and
				determine if it's LoginAsUser or LoginAsMachine
			*/
			if !shouldLoginAsUser && !shouldLoginAsMachine {
				cli.renderer.Output(
					fmt.Sprintf(
						"%s\n\n%s\n%s\n\n%s\n%s\n%s\n%s\n\n",
						ansi.Bold("✪ Welcome to the Auth0 CLI 🎊"),
						"An Auth0 tenant is required to operate this CLI.",
						"To create one, visit: https://auth0.com/signup.",
						"You may authenticate to your tenant either as a user with personal",
						"credentials or as a machine via client credentials. For more",
						"information about authenticating the CLI to your tenant, visit",
						"the docs: https://auth0.github.io/auth0-cli/auth0_login.html",
					),
				)

				label := "How would you like to authenticate?"
				help := fmt.Sprintf(
					"%s\n%s\n",
					"Authenticating as a user is recommended if performing ad-hoc operations or working locally.",
					"Alternatively, authenticating as a machine is recommended for automated workflows (ex:CI).",
				)
				input := prompt.SelectInput("", label, help, []string{loginAsUser, loginAsMachine}, loginAsUser, true)
				if err := prompt.AskOne(input, &selectedLoginType); err != nil {
					return handleInputError(err)
				}
			}

			ctx := cmd.Context()

			if shouldLoginAsUser || selectedLoginType == loginAsUser {
				if _, err := RunLoginAsUser(ctx, cli, inputs.AdditionalScopes, inputs.Domain); err != nil {
					return fmt.Errorf("failed to start the authentication process: %w", err)
				}
			} else {
				if err := RunLoginAsMachine(ctx, inputs, cli, cmd); err != nil {
					return err
				}
			}

			cli.tracker.TrackCommandRun(cmd, cli.Config.InstallID)

			if len(cli.Config.Tenants) > 1 {
				cli.renderer.Infof("%s Switch between authenticated tenants with `auth0 tenants use <tenant>`",
					ansi.Faint("Hint:"),
				)
			}

			return nil
		},
	}

	loginTenantDomain.RegisterString(cmd, &inputs.Domain, "")
	loginClientID.RegisterString(cmd, &inputs.ClientID, "")
	loginClientSecret.RegisterString(cmd, &inputs.ClientSecret, "")
	loginTenantProfileLabel.RegisterString(cmd, &inputs.TenantProfile, "")
	loginAdditionalScopes.RegisterStringSlice(cmd, &inputs.AdditionalScopes, []string{})
	cmd.MarkFlagsMutuallyExclusive("client-id", "scopes")
	cmd.MarkFlagsMutuallyExclusive("client-secret", "scopes")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}

func ensureAuth0URL(input string) (string, error) {
	if input == "" {
		return "https://*.auth0.com/api/v2/", nil
	}
	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimSuffix(input, "/api/v2")

	// Check if the input ends with auth0.com .
	if !strings.HasSuffix(input, "auth0.com") {
		return "", fmt.Errorf("not a valid auth0.com domain")
	}

	// Extract the domain part without any path.
	domainParts := strings.Split(input, "/")
	domain := domainParts[0]

	// Return the formatted URL.
	return fmt.Sprintf("https://%s/api/v2/", domain), nil
}

// RunLoginAsUser runs the login flow guiding the user through the process
// by showing the login instructions, opening the browser.
func RunLoginAsUser(ctx context.Context, cli *cli, additionalScopes []string, domain string) (config.Tenant, error) {
	domain, err := ensureAuth0URL(domain)
	if err != nil {
		return config.Tenant{}, err
	}

	state, err := auth.GetDeviceCode(ctx, http.DefaultClient, additionalScopes, domain)
	if err != nil {
		return config.Tenant{}, fmt.Errorf("failed to get the device code: %w", err)
	}

	message := fmt.Sprintf("\n%s\n\n",
		"Verify "+ansi.Bold(state.UserCode)+" code in opened browser window to complete authentication.",
	)
	cli.renderer.Output(message)

	if cli.noInput {
		message = "Open the following URL in a browser: %s\n"
		cli.renderer.Infof(message, ansi.Green(state.VerificationURI))
	} else {
		message = "%s to open the browser to log in or %s to quit..."
		cli.renderer.Infof(message, ansi.Green("Press Enter"), ansi.Red("^C"))

		if _, err = fmt.Scanln(); err != nil {
			return config.Tenant{}, err
		}

		if err = browser.OpenURL(state.VerificationURI); err != nil {
			message = "Couldn't open the URL, please do it manually: %s."
			cli.renderer.Warnf(message, state.VerificationURI)
		}
	}

	var result auth.Result
	err = ansi.Spinner("Waiting for the login to complete in the browser", func() error {
		result, err = auth.WaitUntilUserLogsIn(ctx, http.DefaultClient, state)
		return err
	})
	if err != nil {
		return config.Tenant{}, fmt.Errorf("login error: %w", err)
	}

	cli.renderer.Newline()
	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s", result.Domain)
	cli.renderer.Newline()

	tenant := config.Tenant{
		Name:      result.Tenant,
		Domain:    result.Domain,
		ExpiresAt: result.ExpiresAt,
		Scopes:    append(auth.RequiredScopes, additionalScopes...),
	}

	if err := keyring.StoreRefreshToken(result.Domain, result.RefreshToken); err != nil {
		cli.renderer.Warnf("Could not store the access token and the refresh token to the keyring: %s", err)
		cli.renderer.Warnf("Expect to login again when your access token expires.")
	}

	if err := keyring.StoreAccessToken(result.Domain, result.AccessToken); err != nil {
		// In case we don't have a keyring, we want the
		// access token to be saved in the config file.
		tenant.AccessToken = result.AccessToken
	}

	err = cli.Config.AddTenant(tenant)
	if err != nil {
		return config.Tenant{}, fmt.Errorf("failed to add the tenant to the config: %w", err)
	}

	cli.tracker.TrackFirstLogin(cli.Config.InstallID)

	if cli.Config.DefaultTenant != result.Domain {
		message = fmt.Sprintf(
			"Your default tenant is %s. Do you want to change it to %s?",
			cli.Config.DefaultTenant,
			result.Domain,
		)
		if confirmed := prompt.Confirm(message); !confirmed {
			return config.Tenant{}, nil
		}

		if err := cli.Config.SetDefaultTenant(result.Domain); err != nil {
			message = "Failed to set the default tenant, please try 'auth0 tenants use %s' instead: %w"
			cli.renderer.Warnf(message, result.Domain, err)
		}
	}

	return tenant, nil
}

// RunLoginAsMachine facilitates the authentication process using client credentials (client ID, client secret).
func RunLoginAsMachine(ctx context.Context, inputs LoginInputs, cli *cli, cmd *cobra.Command) error {
	if inputs.TenantProfile == "" {
		err := loadCredentials(cmd, inputs)
		if err != nil {
			return err
		}
	}

	token, err := auth.GetAccessTokenFromClientCreds(
		ctx,
		auth.ClientCredentials{
			ClientID:     inputs.ClientID,
			ClientSecret: inputs.ClientSecret,
			Domain:       inputs.Domain,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to fetch access token using client credentials. \n\nEnsure that the provided client-id, client-secret and domain are correct. \n\nerror: %w", err)
	}

	tenant := config.Tenant{
		Name:      strings.Split(inputs.Domain, ".")[0],
		Domain:    inputs.Domain,
		ExpiresAt: token.ExpiresAt,
		ClientID:  inputs.ClientID,
	}

	if err = keyring.StoreClientSecret(inputs.Domain, inputs.ClientSecret); err != nil {
		cli.renderer.Warnf("Could not store the client secret and the access token to the keyring: %s", err)
		cli.renderer.Warnf("Expect to login again when your access token expires.")
	}

	if err := keyring.StoreAccessToken(inputs.Domain, token.AccessToken); err != nil {
		// In case we don't have a keyring, we want the
		// access token to be saved in the config file.
		tenant.AccessToken = token.AccessToken
	}

	if err = cli.Config.AddTenant(tenant); err != nil {
		return fmt.Errorf("failed to save tenant data: %w", err)
	}

	cli.renderer.Newline()
	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s", inputs.Domain)

	cli.tracker.TrackFirstLogin(cli.Config.InstallID)

	return nil
}

func loadCredentials(cmd *cobra.Command, inputs LoginInputs) error {
	if err := loginTenantDomain.Ask(cmd, &inputs.Domain, nil); err != nil {
		return err
	}

	if err := loginClientID.Ask(cmd, &inputs.ClientID, nil); err != nil {
		return err
	}

	if err := loginClientSecret.AskPassword(cmd, &inputs.ClientSecret); err != nil {
		return err
	}

	return nil
}
