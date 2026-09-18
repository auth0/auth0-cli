package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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

	loginClientAssertionPrivateKey = Flag{
		Name:         "Client Assertion Private Key",
		LongForm:     "client-assertion-private-key",
		Help:         "Client Assertion Private key with either a file path or direct content when authenticating via Private key JWT.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginClientAssertionSigningAlg = Flag{
		Name:         "Client Assertion Signing Algorithm",
		LongForm:     "client-assertion-signing-alg",
		Help:         "Client Assertion Signing Algorithm when authenticating via Private key JWT. Supported algorithms: RS256, RS384, PS256.",
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
)

type LoginInputs struct {
	Domain                    string
	ClientID                  string
	ClientSecret              string
	ClientAssertionPrivateKey string
	ClientAssertionSigningAlg string
	AdditionalScopes          []string
}

func (i *LoginInputs) isLoggingInWithAdditionalScopes() bool {
	return len(i.AdditionalScopes) > 0
}

func loginCmd(cli *cli) *cobra.Command {
	var inputs LoginInputs

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long: "Authenticates the Auth0 CLI using either personal credentials (user login) or client credentials (machine login)." +
			"\n\nUse user login on personal machines or interactive environments (not supported for Private Cloud users).\n" +
			"Use machine login for servers, CI, AI agents, or any non-interactive environments — " +
			"this is the recommended method for Private Cloud users and for agent mode.\n\n" +
			"In agent mode, machine login is preferred because it needs no browser. If you run user login in agent mode, " +
			"the CLI emits the device verification URL and code as a JSON object on stdout so an agent can hand them to a human to finish in a browser. " +
			"Agent-mode login also sets the newly authenticated tenant as the default automatically.\n\n",
		Example: `  auth0 login
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-assertion-signing-alg RS256 --client-assertion-private-key <path-to-private-key>
  auth0 login --domain <tenant-domain> --client-id <client-id> --client-assertion-signing-alg RS256 --client-assertion-private-key <client-assertion-private-key>
  auth0 login --scopes "read:client_grants,create:client_grants"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			const (
				loginAsUser    = "As a user"
				loginAsMachine = "As a machine"
				clientSecret   = "Client Secret"
				clientJWT      = "Client Assertion"
			)

			var (
				shouldLoginAsUser          = false
				shouldLoginAsMachineJWT    = false
				shouldLoginAsMachineSecret = false
				shouldLoginAsMachine       = false
				selectedLoginType          = ""
			)

			/*
				Based on the initial inputs we'd like to determine if
				it's a machine login or a user login
				If we successfully determine it, we don't need to prompt the user.

				The --no-input flag add strict restriction that we shall not take any further input after
				initial command.
				Hence, the flow diverges into two based on no-input flag's value.
			*/
			if cli.noInput {
				switch {
				case inputs.Domain != "" && inputs.ClientID != "" && inputs.ClientSecret != "":
					shouldLoginAsMachineSecret = true
				case inputs.Domain != "" && inputs.ClientID != "" && inputs.ClientAssertionSigningAlg != "" && inputs.ClientAssertionPrivateKey != "":
					shouldLoginAsMachineJWT = true
				case inputs.Domain != "" &&
					inputs.ClientID == "" && inputs.ClientSecret == "" &&
					inputs.ClientAssertionSigningAlg == "" && inputs.ClientAssertionPrivateKey == "":
					shouldLoginAsUser = true
				case inputs.Domain != "" || inputs.ClientID != "" || inputs.ClientSecret != "" || inputs.ClientAssertionSigningAlg != "" || inputs.ClientAssertionPrivateKey != "":
					return usageError{err: fmt.Errorf("for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)"), reason: "missing_required_flags"}
				default:
					/*
						If no flags are passed along with --no-input, it is defaulted to user login flow.
					*/
					shouldLoginAsUser = true
				}
			} else {
				if inputs.ClientAssertionSigningAlg != "" || inputs.ClientAssertionPrivateKey != "" {
					shouldLoginAsMachineJWT = true
				}
				if inputs.ClientSecret != "" {
					shouldLoginAsMachineSecret = true
				}
				if inputs.ClientID != "" {
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
			if !shouldLoginAsUser && !shouldLoginAsMachineSecret && !shouldLoginAsMachineJWT && !shouldLoginAsMachine {
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

				promptText := prompt.SelectInput(
					"", "How would you like to authenticate?",
					"Authenticating as a user is recommended for local use.\nMachine auth is recommended for CI/CD.",
					[]string{loginAsUser, loginAsMachine}, loginAsUser, true,
				)
				if err := prompt.AskOne(promptText, &selectedLoginType); err != nil {
					return handleInputError(err)
				}
			}

			switch {
			case shouldLoginAsUser || selectedLoginType == loginAsUser:
				if _, err := RunLoginAsUser(ctx, cli, inputs.AdditionalScopes, inputs.Domain); err != nil {
					return fmt.Errorf("failed to start user login: %w", err)
				}
			default:
				if err := loginTenantDomain.Ask(cmd, &inputs.Domain, nil); err != nil {
					return err
				}

				// Prompt client credentials method if not clear yet.
				if !shouldLoginAsMachineSecret && !shouldLoginAsMachineJWT {
					promptText := prompt.SelectInput(
						"", "How would you like to provide client credentials?",
						"", []string{clientSecret, clientJWT}, clientSecret, true,
					)
					if err := prompt.AskOne(promptText, &selectedLoginType); err != nil {
						return handleInputError(err)
					}
					if selectedLoginType == clientJWT {
						shouldLoginAsMachineJWT = true
					} else {
						shouldLoginAsMachineSecret = true
					}
				}

				if shouldLoginAsMachineJWT {
					if err := RunLoginAsMachineJWT(ctx, inputs, cli, cmd); err != nil {
						return fmt.Errorf("failed to start JWT machine login: %w", err)
					}
				} else if shouldLoginAsMachineSecret {
					if err := RunLoginAsMachineSecret(ctx, inputs, cli, cmd); err != nil {
						return fmt.Errorf("failed to start secret machine login: %w", err)
					}
				}
			}

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
	loginClientAssertionSigningAlg.RegisterString(cmd, &inputs.ClientAssertionSigningAlg, "")
	loginClientAssertionPrivateKey.RegisterString(cmd, &inputs.ClientAssertionPrivateKey, "")
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
		return "", validationError{err: fmt.Errorf("not a valid auth0.com domain"), reason: "invalid_flag_value"}
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

	var result auth.Result

	if cli.renderer.AgentMode {
		// An agent has no browser to open and no stdin to press Enter on, so the
		// interactive device-code prompts (and the stderr spinner) would strand it.
		// Instead emit the verification details as a single JSON object to stdout so
		// the agent can relay the link and code to a human, then poll below until
		// that human approves in a browser or the device code expires.
		details, marshalErr := json.Marshal(struct {
			VerificationURI string `json:"verification_uri"`
			UserCode        string `json:"user_code"`
			ExpiresIn       int    `json:"expires_in"`
			Interval        int    `json:"interval"`
		}{
			VerificationURI: state.VerificationURI,
			UserCode:        state.UserCode,
			ExpiresIn:       state.ExpiresIn,
			Interval:        state.Interval,
		})
		if marshalErr != nil {
			return config.Tenant{}, fmt.Errorf("failed to encode device login details: %w", marshalErr)
		}
		cli.renderer.OutputPreformattedJSON(string(details))

		if result, err = auth.WaitUntilUserLogsIn(ctx, http.DefaultClient, state); err != nil {
			return config.Tenant{}, fmt.Errorf("login error: %w", err)
		}
	} else {
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

		err = ansi.Spinner("Waiting for the login to complete in the browser", func() error {
			result, err = auth.WaitUntilUserLogsIn(ctx, http.DefaultClient, state)
			return err
		})
		if err != nil {
			return config.Tenant{}, fmt.Errorf("login error: %w", err)
		}
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

	if err := keyring.StoreAccessToken(result.Domain, result.AccessToken); err != nil {
		// In case we don't have a keyring, we want the
		// access token to be saved in the config file.
		tenant.AccessToken = result.AccessToken
	}

	err = cli.Config.AddTenant(tenant)
	if err != nil {
		return config.Tenant{}, fmt.Errorf("failed to add the tenant to the config: %w", err)
	}

	cli.tracker.TrackFirstLogin(cli.Config.InstallID, "As-User")

	if cli.renderer.AgentMode {
		// The "Successfully logged in" lines above are suppressed in agent mode, so
		// emit a final machine-readable confirmation on stdout. This closes the
		// stream that opened with the device verification object.
		details, marshalErr := json.Marshal(struct {
			LoggedIn bool   `json:"logged_in"`
			Tenant   string `json:"tenant"`
			Domain   string `json:"domain"`
		}{
			LoggedIn: true,
			Tenant:   result.Tenant,
			Domain:   result.Domain,
		})
		if marshalErr != nil {
			return config.Tenant{}, fmt.Errorf("failed to encode login result: %w", marshalErr)
		}
		cli.renderer.OutputPreformattedJSON(string(details))
	}

	if cli.Config.DefaultTenant != result.Domain {
		// In agent mode there is no human to answer this prompt, and running login is
		// an explicit request to use this tenant, so switch the default automatically
		// instead of prompting. Prompting would either strand the login on the old
		// default (closed stdin answers "no") or block on an interactive prompt.
		if cli.renderer.AgentMode {
			// The login already succeeded and {"logged_in":true,...} was emitted on
			// stdout, so a failure to switch the default tenant is a convenience step
			// that must not turn a successful login into a non-zero exit. Mirror the
			// human path and treat it as non-fatal. Warnf is suppressed in agent mode,
			// keeping stderr clean while the success object stands.
			if err := cli.Config.SetDefaultTenant(result.Domain); err != nil {
				cli.renderer.Warnf("Failed to set the default tenant, run 'auth0 tenants use %s' to switch: %v", result.Domain, err)
			}
			return tenant, nil
		}

		message := fmt.Sprintf(
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

// RunLoginAsMachineSecret facilitates the authentication process using client credentials (client ID, client secret).
func RunLoginAsMachineSecret(ctx context.Context, inputs LoginInputs, cli *cli, cmd *cobra.Command) error {
	if err := loginClientID.Ask(cmd, &inputs.ClientID, nil); err != nil {
		return err
	}

	if err := loginClientSecret.AskPassword(cmd, &inputs.ClientSecret); err != nil {
		return err
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

	if err = keyring.StoreClientSecret(inputs.Domain, inputs.ClientSecret); err != nil {
		cli.renderer.Warnf("Could not store the client secret and the access token to the keyring: %s", err)
		cli.renderer.Warnf("Expect to login again when your access token expires.")
	}

	tenant := config.Tenant{
		Name:      strings.Split(inputs.Domain, ".")[0],
		Domain:    inputs.Domain,
		ExpiresAt: token.ExpiresAt,
		ClientID:  inputs.ClientID,
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

	cli.tracker.TrackFirstLogin(cli.Config.InstallID, "As-Machine")

	return nil
}

// RunLoginAsMachineJWT facilitates the authentication process using  the client credentials
// with Private Key JWT authentication flow. (client ID, client Assertion Private key, client Assertion Signing Algorithm).
func RunLoginAsMachineJWT(ctx context.Context, inputs LoginInputs, cli *cli, cmd *cobra.Command) error {
	if err := loginClientID.Ask(cmd, &inputs.ClientID, nil); err != nil {
		return err
	}

	if err := loginClientAssertionSigningAlg.Select(cmd, &inputs.ClientAssertionSigningAlg, []string{"RS256", "RS384", "PS256"}, nil); err != nil {
		return err
	}

	if err := loginClientAssertionPrivateKey.Ask(cmd, &inputs.ClientAssertionPrivateKey, nil); err != nil {
		return err
	}

	domain := "https://" + inputs.Domain

	if !strings.HasPrefix(inputs.ClientAssertionPrivateKey, "-----BEGIN ") {
		key, err := readPrivateKey(inputs.ClientAssertionPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to read the private key file: %w", err)
		}
		inputs.ClientAssertionPrivateKey = key
	}

	token, err := auth.GetAccessTokenFromClientPrivateJWT(
		auth.PrivateKeyJwtTokenSource{
			Ctx:                       ctx,
			ClientID:                  inputs.ClientID,
			ClientAssertionSigningAlg: inputs.ClientAssertionSigningAlg,
			URI:                       domain,
			Audience:                  domain + "/api/v2/",
			ClientAssertionPrivateKey: inputs.ClientAssertionPrivateKey,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to fetch access token using client credentials with Private Key. \n\nEnsure that the provided client-id, client-assertion-private-key, client-assertion-signing-alg and domain are correct. \n\nerror: %w", err)
	}

	tenant := config.Tenant{
		Name:      strings.Split(inputs.Domain, ".")[0],
		Domain:    inputs.Domain,
		ExpiresAt: token.ExpiresAt,
		ClientID:  inputs.ClientID,
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

	cli.tracker.TrackFirstLogin(cli.Config.InstallID, "As-Machine")

	return nil
}

func readPrivateKey(path string) (string, error) {
	// Read the content of the file.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// Return the content.
	return string(content), nil
}
