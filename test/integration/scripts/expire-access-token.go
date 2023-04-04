package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/auth0/auth0-cli/internal/cli"
)

func main() {
	tenantDomain := os.Getenv("AUTH0_CLI_CLIENT_DOMAIN")

	configFilePath := path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")

	var buf []byte
	buf, err := os.ReadFile(configFilePath)
	if err != nil {
		fmt.Println("Cannot read config file: ", err)
		os.Exit(1)
		return
	}

	var config cli.Config

	if err := json.Unmarshal(buf, &config); err != nil {
		fmt.Println("Cannot unmarshal config file:", err)
		os.Exit(1)
		return
	}

	configWithExpiredToken := config.Tenants[tenantDomain]
	configWithExpiredToken.ExpiresAt = time.Now().Add(-1 * time.Minute)
	config.Tenants[tenantDomain] = configWithExpiredToken

	err = cli.UpdateConfigFile(config, configFilePath)
	if err != nil {
		fmt.Println("Error writing updated config file:", err)
		os.Exit(1)
		return
	}

	os.Exit(0)
}
