package cli

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

// QuickStart app types and defaults
const (
	qsNative    = "native"
	qsSpa       = "spa"
	qsWebApp    = "webapp"
	qsBackend   = "backend"
	_defaultURL = "http://localhost:3000"
)

var (
	//go:embed data/quickstarts.json
	qsBuf             []byte
	quickstartsByType = func() (qs map[string][]quickstart) {
		if err := json.Unmarshal(qsBuf, &qs); err != nil {
			panic(err)
		}
		return
	}()

	clientID = Flag{
		Name:       "ClientID",
		LongForm:   "client-id",
		ShortForm:  "c",
		Help:       "Client Id of an Auth0 application.",
		IsRequired: true,
	}

	stack = Flag{
		Name:      "Stack",
		LongForm:  "stack",
		ShortForm: "s",
		Help:      "Tech/Language of the quickstart sample to download",
	}
)

type quickstart struct {
	Name    string   `json:"name"`
	Samples []string `json:"samples"`
	Org     string   `json:"org"`
	Repo    string   `json:"repo"`
	Branch  string   `json:"branch,omitempty"`
}

func quickstartsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quickstarts",
		Short:   "Quickstart support for getting bootstrapped",
		Aliases: []string{"qs"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(downloadQuickstart(cli))

	return cmd
}

func downloadQuickstart(cli *cli) *cobra.Command {
	var inputs struct {
		ClientID string
		Stack    string
	}

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a quickstart sample app for a specific tech stack",
		Long:  `auth0 quickstarts download --client-id <client-id> --stack <stack>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !canPrompt(cmd) {
				return errors.New("This command can only be run on interactive mode")
			}

			if err := clientID.Ask(cmd, &inputs.ClientID); err != nil {
				return err
			}

			client, err := cli.api.Client.Read(inputs.ClientID)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred, please verify your client id: %v", err.Error())
			}

			if inputs.Stack == "" {
				// get the valid types for this App:
				stacks, err := quickstartStacksFromType(client.GetAppType())
				if err != nil {
					return fmt.Errorf("An unexpected error occurred: %v", err)
				}
				// ask for input using the valid types only:
				if err := stack.Select(cmd, &inputs.Stack, stacks); err != nil {
					return err
				}
			}

			target, exists, err := quickstartPathFor(client)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred: %v", err)
			}

			if exists && !cli.force {
				if confirmed := prompt.Confirm(fmt.Sprintf("WARNING: %s already exists. Are you sure you want to proceed?", target)); !confirmed {
					return nil
				}
			}

			q, err := getQuickstart(client.GetAppType(), inputs.Stack)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred with the specified stack %v: %v", inputs.Stack, err)
			}

			err = ansi.Spinner("Downloading quickstart sample", func() error {
				return downloadQuickStart(cmd.Context(), cli, client, target, q)
			})

			if err != nil {
				return fmt.Errorf("Unable to download quickstart sample: %v", err)
			}

			cli.renderer.Infof("Quickstart sample sucessfully downloaded at %s", target)

			qsType := quickstartsTypeFor(client.GetAppType())
			if err := promptDefaultURLs(cmd.Context(), cli, client, qsType); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	clientID.RegisterString(cmd, &inputs.ClientID, "")
	stack.RegisterString(cmd, &inputs.Stack, "")
	return cmd
}

const (
	quickstartEndpoint           = `https://auth0.com/docs/package/v2`
	quickstartContentType        = `application/json`
	quickstartOrg                = "auth0-samples"
	quickstartDefaultCallbackURL = `https://YOUR_APP/callback`
)

func downloadQuickStart(ctx context.Context, cli *cli, client *management.Client, target string, q quickstart) error {
	var payload struct {
		Branch       string `json:"branch"`
		Org          string `json:"org"`
		Repo         string `json:"repo"`
		Path         string `json:"path"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		CallbackURL  string `json:"callback_url"`
		Domain       string `json:"domain"`
		Tenant       string `json:"tenant"`
	}

	ten, err := cli.getTenant()
	if err != nil {
		return fmt.Errorf("Unable to get tenant: %v", err)
	}

	payload.Tenant = ten.Name
	payload.Domain = ten.Domain

	// FIXME(copland): Default to first item from list of samples.
	// Eventually we should add a forced survey for user to select one if
	// there are multiple.
	payload.Branch = q.Branch
	payload.Repo = q.Repo
	payload.Path = q.Samples[0]

	// These appear to be largely constant and refers to the github
	// username they're under.
	payload.Org = quickstartOrg
	payload.ClientID = client.GetClientID()
	payload.ClientSecret = client.GetClientSecret()

	// Callback URL, if not set, will just take the default one.
	payload.CallbackURL = quickstartDefaultCallbackURL
	if list := urlsFor(client.Callbacks); len(list) > 0 {
		payload.CallbackURL = list[0]
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err.Error())
	}

	req, err := http.NewRequest("POST", quickstartEndpoint, buf)
	if err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}
	req.Header.Set("Content-Type", quickstartContentType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	tmpfile, err := ioutil.TempFile("", "auth0-quickstart*.zip")
	if err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}

	_, err = io.Copy(tmpfile, res.Body)
	if err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}

	if err := archiver.Unarchive(tmpfile.Name(), target); err != nil {
		return fmt.Errorf("An unexpected error occurred: %v", err)
	}

	return nil
}

func quickstartPathFor(client *management.Client) (p string, exists bool, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	re := regexp.MustCompile(`[^\w]+`)
	friendlyName := re.ReplaceAllString(client.GetName(), "-")
	target := path.Join(wd, friendlyName)

	exists = true
	if _, err := os.Stat(target); err != nil {
		if !os.IsNotExist(err) {
			return "", false, err
		}
		exists = false
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return "", false, err
	}

	return target, exists, nil
}

func getQuickstart(t, stack string) (quickstart, error) {
	qsType := quickstartsTypeFor(t)
	quickstarts, ok := quickstartsByType[qsType]
	if !ok {
		return quickstart{}, fmt.Errorf("Unknown quickstart type: %s", qsType)
	}
	for _, q := range quickstarts {
		if q.Name == stack {
			return q, nil
		}
	}
	return quickstart{}, fmt.Errorf("Quickstart not found for %s/%s", qsType, stack)
}

func quickstartStacksFromType(t string) ([]string, error) {
	qsType := quickstartsTypeFor(t)
	_, ok := quickstartsByType[qsType]
	if !ok {
		return nil, fmt.Errorf("Unknown quickstart type: %s", qsType)
	}
	stacks := make([]string, 0, len(quickstartsByType[qsType]))
	for _, s := range quickstartsByType[qsType] {
		stacks = append(stacks, s.Name)
	}
	return stacks, nil
}

func quickstartsTypeFor(v string) string {
	switch {
	case v == "native":
		return qsNative
	case v == "spa":
		return qsSpa
	case v == "regular_web":
		return qsWebApp
	case v == "non_interactive":
		return qsBackend
	default:
		return "generic"
	}
}

// promptDefaultURLs checks whether the application is SPA or WebApp and
// whether the app has already added the default quickstart url to allowed url lists.
// If not, it prompts the user to add the default url and updates the application
// if they accept.
func promptDefaultURLs(ctx context.Context, cli *cli, client *management.Client, qsType string) error {
	if !strings.EqualFold(qsType, qsSpa) && !strings.EqualFold(qsType, qsWebApp) {
		return nil
	}
	if containsStr(client.Callbacks, _defaultURL) || containsStr(client.AllowedLogoutURLs, _defaultURL) {
		return nil
	}

	a := &management.Client{
		Callbacks:         client.Callbacks,
		WebOrigins:        client.WebOrigins,
		AllowedLogoutURLs: client.AllowedLogoutURLs,
	}

	if confirmed := prompt.Confirm(formatURLPrompt(qsType)); confirmed {
		a.Callbacks = append(a.Callbacks, _defaultURL)
		a.AllowedLogoutURLs = append(a.AllowedLogoutURLs, _defaultURL)
		if strings.EqualFold(qsType, qsSpa) {
			a.WebOrigins = append(a.WebOrigins, _defaultURL)
		}

		err := ansi.Spinner("Updating application", func() error {
			return cli.api.Client.Update(client.GetClientID(), a)
		})
		if err != nil {
			return err
		}
		cli.renderer.Infof("Application successfully updated")
	}
	return nil
}

// formatURLPrompt creates the correct prompt based on app type for
// asking the user if they would like to add default urls.
func formatURLPrompt(qsType string) string {
	var p strings.Builder
	p.WriteString("\nQuickstarts use localhost, do you want to add %s to the list of allowed callback URLs")
	if strings.EqualFold(qsType, qsSpa) {
		p.WriteString(", logout URLs, and web origins?")
	} else {
		p.WriteString(" and logout URLs?")
	}
	return fmt.Sprintf(p.String(), _defaultURL)
}
