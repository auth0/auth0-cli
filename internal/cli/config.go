package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

var requiredScopes = auth.RequiredScopes()

var desiredInputs = `Config init is intended for non-interactive use, 
ensure the following env variables are set: 

AUTH0_CLI_CLIENT_DOMAIN 
AUTH0_CLI_CLIENT_ID
AUTH0_CLI_CLIENT_SECRET

Interactive logins should use "auth0 login" instead.`

type params struct {
	filePath     string
	clientDomain string
	clientID     string
	clientSecret string
}

func (p params) validate() error {
	if p.clientDomain == "" {
		return fmt.Errorf("Missing client domain.\n%s", desiredInputs)
	}

	u, err := url.Parse(p.clientDomain)
	if err != nil {
		return fmt.Errorf("Failed to parse client domain: %s", p.clientDomain)
	}

	if u.Scheme != "" {
		return fmt.Errorf("Client domain cant include a scheme: %s", p.clientDomain)
	}

	if p.clientID == "" {
		return fmt.Errorf("Missing client id.\n%s", desiredInputs)
	}

	if p.clientSecret == "" {
		return fmt.Errorf("Missing client secret.\n%s", desiredInputs)
	}
	return nil
}

func configCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage auth0-cli config",
		Long:  "Manage auth0-cli config",
	}

	cmd.AddCommand(initCmd(cli))
	return cmd
}

func initCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize valid cli config from environment variables",
		RunE: func(command *cobra.Command, args []string) error {
			filePath := viper.GetString("FILEPATH")
			clientDomain := viper.GetString("CLIENT_DOMAIN")
			clientID := viper.GetString("CLIENT_ID")
			clientSecret := viper.GetString("CLIENT_SECRET")

			cli.setPath(filePath)
			p := params{filePath, clientDomain, clientID, clientSecret}
			if err := p.validate(); err != nil {
				return err
			}

			u, err := url.Parse("https://" + p.clientDomain)
			if err != nil {
				return err
			}

			// integration test client doesn't have openid or offline_access scopes granted
			scopesForTest := requiredScopes[2:]
			c := &clientcredentials.Config{
				ClientID:     p.clientID,
				ClientSecret: p.clientSecret,
				TokenURL:     u.String() + "/oauth/token",
				EndpointParams: url.Values{
					"client_id": {p.clientID},
					"scope":     {strings.Join(scopesForTest, " ")},
					"audience":  {u.String() + "/api/v2/"},
				},
			}

			token, err := c.Token(context.Background())
			if err != nil {
				return err
			}

			t := tenant{
				Name:        p.clientDomain,
				Domain:      p.clientDomain,
				AccessToken: token.AccessToken,
				ExpiresAt:   token.Expiry,
				Scopes:      requiredScopes,
			}

			if err := cli.addTenant(t); err != nil {
				return fmt.Errorf("Unexpected error adding tenant to config: %w", err)
			}
			return nil
		},
	}
	viper.SetEnvPrefix("AUTH0_CLI")
	viper.AutomaticEnv()

	flags := cmd.Flags()
	flags.String("filepath", path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json"), "Filepath for the auth0 cli config")
	_ = viper.BindPFlag("FILEPATH", flags.Lookup("filepath"))
	flags.String("client-id", "", "Client ID to set within config")
	_ = viper.BindPFlag("CLIENT_ID", flags.Lookup("client-id"))
	flags.String("client-secret", "", "Client secret to use to generate token which is set within config")
	_ = viper.BindPFlag("CLIENT_SECRET", flags.Lookup("client-secret"))
	flags.String("client-domain", "", "Client domain to use to generate token which is set within config")
	_ = viper.BindPFlag("CLIENT_DOMAIN", flags.Lookup("client-domain"))

	return cmd
}
