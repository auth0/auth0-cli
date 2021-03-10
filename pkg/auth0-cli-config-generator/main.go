package main

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

	"github.com/lestrrat-go/jwx/jwt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/clientcredentials"
)

type params struct {
	filePath     string
	clientName   string
	clientDomain string
	clientID     string
	clientSecret string
}

func (p params) validate() error {
	if p.clientName == "" {
		return fmt.Errorf("Missing client name")
	}

	if p.clientDomain == "" {
		return fmt.Errorf("Missing client domain")
	}

	u, err := url.Parse(p.clientDomain)
	if err != nil {
		return fmt.Errorf("Failed to parse client domain: %s", p.clientDomain)
	}

	if u.Scheme != "" {
		return fmt.Errorf("Client domain cant include a scheme: %s", p.clientDomain)
	}

	if p.clientID == "" {
		return fmt.Errorf("Missing client id")
	}

	if p.clientSecret == "" {
		return fmt.Errorf("Missing client secret")
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

func isLoggedIn(filePath string) bool {
	var c config
	var buf []byte
	var err error
	if buf, err = ioutil.ReadFile(filePath); err != nil {
		return false
	}

	if err := json.Unmarshal(buf, &c); err != nil {
		return false
	}

	if c.Tenants == nil {
		return false
	}

	if c.DefaultTenant == "" {
		return false
	}

	t, err := jwt.ParseString(c.Tenants[c.DefaultTenant].AccessToken)
	if err != nil {
		return false
	}

	if err = jwt.Validate(t, jwt.WithIssuer("https://auth0.auth0.com/")); err != nil {
		return false
	}

	return true
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

	if err = ioutil.WriteFile(filePath, buf, 0600); err != nil {
		return err
	}

	return nil
}

func main() {
	var cmd = &cobra.Command{
		Use: "auth0-cli-config-generator",
		RunE: func(command *cobra.Command, args []string) error {
			reuseConfig := viper.GetBool("REUSE_CONFIG")
			filePath := viper.GetString("FILEPATH")
			clientName := viper.GetString("CLIENT_NAME")
			clientDomain := viper.GetString("CLIENT_DOMAIN")
			clientID := viper.GetString("CLIENT_ID")
			clientSecret := viper.GetString("CLIENT_SECRET")

			if reuseConfig {
				if !isLoggedIn(filePath) {
					return fmt.Errorf("Config file is not valid: %s", filePath)
				}
				fmt.Println("Reusing valid config file")
				return nil
			}

			p := params{filePath, clientName, clientDomain, clientID, clientSecret}
			if err := p.validate(); err != nil {
				return err
			}

			u, err := url.Parse("https://" + p.clientDomain)
			if err != nil {
				return err
			}

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

			t := tenant{p.clientName, p.clientDomain, token.AccessToken, token.Expiry}

			cfg := config{p.clientName, map[string]tenant{p.clientName: t}}
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
	flags.String("client-name", "", "")
	_ = viper.BindPFlag("CLIENT_NAME", flags.Lookup("client-name"))
	flags.String("client-id", "", "")
	_ = viper.BindPFlag("CLIENT_ID", flags.Lookup("client-id"))
	flags.String("client-secret", "", "")
	_ = viper.BindPFlag("CLIENT_SECRET", flags.Lookup("client-secret"))
	flags.String("domain", "", "")
	_ = viper.BindPFlag("DOMAIN", flags.Lookup("domain"))
	flags.Bool("reuse-config", true, "Reuse an existing config file if found")
	_ = viper.BindPFlag("REUSE_CONFIG", flags.Lookup("reuse-config"))

	_ = cmd.Execute()
}
