package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/display"
	"gopkg.in/auth0.v5/management"
)

type data struct {
	DefaultTenant string            `json:"default_tenant"`
	Tenants       map[string]tenant `json:"tenants"`
}

type tenant struct {
	Domain string `json:"domain"`

	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`

	// TODO(cyx): This will be what we do with device flow.
	BearerToken string `json:"bearer_token,omitempty"`
}

type cli struct {
	api      *management.Management
	renderer *display.Renderer

	verbose bool
	tenant  string
	format  string

	initOnce sync.Once
	path     string
	data     data
}

func (c *cli) setup() error {
	t, err := c.getTenant()
	if err != nil {
		return err
	}

	if t.BearerToken != "" {
		c.api, err = management.New(t.Domain,
			management.WithStaticToken(t.BearerToken),
			management.WithDebug(c.verbose))
	} else {
		c.api, err = management.New(t.Domain,
			management.WithClientCredentials(t.ClientID, t.ClientSecret),
			management.WithDebug(c.verbose))
	}

	return err
}

func (c *cli) getTenant() (tenant, error) {
	if err := c.init(); err != nil {
		return tenant{}, err
	}

	t, ok := c.data.Tenants[c.tenant]
	if !ok {
		return tenant{}, fmt.Errorf("Unable to find tenant: %s", c.tenant)
	}

	return t, nil
}

func (c *cli) init() error {
	var err error
	c.initOnce.Do(func() {
		if c.path == "" {
			c.path = path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
		}

		var buf []byte
		if buf, err = ioutil.ReadFile(c.path); err != nil {
			return
		}

		if err = json.Unmarshal(buf, &c.data); err != nil {
			return
		}

		if c.tenant == "" && c.data.DefaultTenant == "" {
			err = fmt.Errorf("Not yet configured. Try `auth0 login`.")
			return
		}

		if c.tenant == "" {
			c.tenant = c.data.DefaultTenant
		}

		format := strings.ToLower(c.format)
		if format != "" && format != string(display.OutputFormatJSON) {
			err = fmt.Errorf("Invalid format. Use `--format=json` or ommit this option to use the default format.")
			return
		}

		c.renderer = &display.Renderer{
			Tenant: c.tenant,
			Writer: os.Stdout,
			Format: display.OutputFormat(format),
		}
	})

	return err
}
