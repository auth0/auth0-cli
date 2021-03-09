package main

// https://github.com/spf13/viper/issues/85

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

type params struct {
	filePath     string
	name         string
	domain       string
	clientID     string
	clientSecret string
}

func (p params) validate() error {
	if p.name == "" {
		return fmt.Errorf("Missing name")
	}

	if p.domain == "" {
		return fmt.Errorf("Missing domain")
	}

	u, err := url.Parse(p.domain)
	if err != nil {
		return fmt.Errorf("Failed to parse domain: %s", p.domain)
	}

	if u.Scheme != "" {
		return fmt.Errorf("Domain cant include a scheme: %s", p.domain)
	}

	if p.clientID == "" {
		return fmt.Errorf("Missing client-id")
	}

	if p.clientSecret == "" {
		return fmt.Errorf("Missing client-secret")
	}
	return nil
}

type config struct {
	DefaultTenant string            `json:"default_tenant"`
	Tenants       map[string]tenant `json:"tenants"`
}

type tenant struct {
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	AccessToken string    `json:"access_token,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func persistConfig(filePath string, c config) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	buf, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, buf, 0600)
}

func main() {
	var cmd = &cobra.Command{
		Use: "auth0-cli-config-generator",
		RunE: func(command *cobra.Command, args []string) error {
			p := params{viper.GetString("FILEPATH"), viper.GetString("NAME"), viper.GetString("DOMAIN"), viper.GetString("CLIENT_ID"), viper.GetString("CLIENT_SECRET")}
			if err := p.validate(); err != nil {
				return err
			}

			u, err := url.Parse(p.domain)
			if err != nil {
				return err
			}
			u.Scheme = "https"

			c := &clientcredentials.Config{
				ClientID:       p.clientID,
				ClientSecret:   p.clientSecret,
				TokenURL:       u.String() + "/oauth/token",
				EndpointParams: url.Values{"audience": {u.String() + "/api/v2/"}},
			}

			token, err := c.Token(context.Background())
			if err != nil {
				return err
			}

			t := tenant{p.name, p.domain, token.AccessToken, token.Expiry}

			cfg := config{p.name, map[string]tenant{p.name: t}}
			if err := persistConfig(p.filePath, cfg); err != nil {
				return err
			}

			return nil
		},
	}
	viper.SetEnvPrefix("AUTH0_CLI")
	viper.AutomaticEnv()

	flags := cmd.Flags()
	flags.String("filepath", path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json"), "Filepath")
	_ = viper.BindPFlag("FILEPATH", flags.Lookup("filepath"))
	flags.String("name", "", "")
	_ = viper.BindPFlag("NAME", flags.Lookup("name"))
	flags.String("client-id", "", "")
	_ = viper.BindPFlag("CLIENT_ID", flags.Lookup("client-id"))
	flags.String("client-secret", "", "")
	_ = viper.BindPFlag("CLIENT_SECRET", flags.Lookup("client-secret"))
	flags.String("domain", "", "")
	_ = viper.BindPFlag("DOMAIN", flags.Lookup("domain"))

	_ = cmd.Execute()
}
