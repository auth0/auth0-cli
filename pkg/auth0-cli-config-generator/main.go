package main

// https://github.com/spf13/viper/issues/85

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

type config struct {
	domain        string
	client_id     string
	client_secret string
}

func (c config) validate() error {
	if c.domain == "" {
		return fmt.Errorf("Missing domain")
	}

	u, err := url.Parse(c.domain)
	if err != nil {
		return fmt.Errorf("Failed to parse domain: %s", c.domain)
	}

	if u.Scheme != "" {
		return fmt.Errorf("Domain cant include a scheme: %s", c.domain)
	}

	if c.client_id == "" {
		return fmt.Errorf("Missing client-id")
	}

	if c.client_secret == "" {
		return fmt.Errorf("Missing client-secret")
	}
	return nil
}

func main() {
	var cmd = &cobra.Command{
		Use: "auth0-cli-config-generator",
		RunE: func(command *cobra.Command, args []string) error {

			cfg := config{viper.GetString("DOMAIN"), viper.GetString("CLIENT_ID"), viper.GetString("CLIENT_SECRET")}
			if err := cfg.validate(); err != nil {
				return err
			}

			u, err := url.Parse(cfg.domain)
			if err != nil {
				return err
			}
			u.Scheme = "https"

			c := &clientcredentials.Config{
				ClientID:       cfg.client_id,
				ClientSecret:   cfg.client_secret,
				TokenURL:       u.String() + "/oauth/token",
				EndpointParams: url.Values{"audience": {u.String() + "/api/v2/"}},
			}

			token, err := c.Token(context.Background())
			if err != nil {
				return err
			}

			fmt.Printf("DEBUG:%#v\n", token.AccessToken)

			return nil
		},
	}
	viper.SetEnvPrefix("AUTH0_CLI")
	viper.AutomaticEnv()

	flags := cmd.Flags()
	flags.String("client-id", "", "")
	viper.BindPFlag("CLIENT_ID", flags.Lookup("client-id"))
	flags.String("client-secret", "", "")
	viper.BindPFlag("CLIENT_SECRET", flags.Lookup("client-secret"))
	flags.String("domain", "", "")
	viper.BindPFlag("DOMAIN", flags.Lookup("domain"))

	cmd.Execute()
}
