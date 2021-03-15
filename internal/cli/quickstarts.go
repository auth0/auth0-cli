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

var (
	//go:embed data/quickstarts.json
	qsBuf             []byte
	quickstartsByType = func() (qs map[string][]quickstart) {
		if err := json.Unmarshal(qsBuf, &qs); err != nil {
			panic(err)
		}
		return
	}()
)

// QuickStart app types and defaults
const (
	QSNative    = "native"
	QSSpa       = "spa"
	QSWebApp    = "webapp"
	QSBackend   = "backend"
	_defaultURL = "http://localhost:3000"
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
	var flags struct {
		ClientID string
		Type     string
		Stack    string
	}

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a quickstart sample app for a specific tech stack",
		Long:  `auth0 quickstarts download --type <type> --client-id <client-id> --stack <stack>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !shouldPrompt(cmd, "client-id") {
				return errors.New("This command can only be run on interactive mode")
			}

			selectedClientID := flags.ClientID

			if selectedClientID == "" {
				input := prompt.TextInput("client-id", "Client Id:", "Client Id of an Auth0 application.", true)
				if err := prompt.AskOne(input, &selectedClientID); err != nil {
					return fmt.Errorf("An unexpected error occurred: %v", err)
				}
			}

			client, err := cli.api.Client.Read(selectedClientID)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred, please verify your client id: %v", err.Error())
			}

			selectedStack := flags.Stack

			if selectedStack == "" {
				stacks, err := quickstartStacksFromType(client.GetAppType())
				if err != nil {
					return fmt.Errorf("An unexpected error occurred: %v", err)
				}
				input := prompt.SelectInput("stack", "Stack:", "Tech/Language of the quickstart sample to download", stacks, true)
				if err := prompt.AskOne(input, &selectedStack); err != nil {
					return fmt.Errorf("An unexpected error occurred: %v", err)
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

			q, err := getQuickstart(client.GetAppType(), selectedStack)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred with the specified stack %v: %v", selectedStack, err)
			}

			err = ansi.Spinner("Downloading quickstart sample", func() error {
				return downloadQuickStart(context.TODO(), cli, client, target, q)
			})

			if err != nil {
				return fmt.Errorf("Unable to download quickstart sample: %v", err)
			}

			cli.renderer.Infof("Quickstart sample sucessfully downloaded at %s", target)

			qsType := quickstartsTypeFor(client.GetAppType())
			if err := promptDefaultURLs(context.TODO(), cli, client, qsType); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&flags.ClientID, "client-id", "c", "", "Client Id of an Auth0 application.")
	cmd.Flags().StringVarP(&flags.Stack, "stack", "s", "", "Tech/Language of the quickstart sample to download.")
	mustRequireFlags(cmd, "client-id")

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
		return QSNative
	case v == "spa":
		return QSSpa
	case v == "regular_web":
		return QSWebApp
	case v == "non_interactive":
		return QSBackend
	default:
		return "generic"
	}
}

// promptDefaultURLs checks whether the application is SPA or WebApp and
// whether the app has already default quickstart url to allowed url lists.
// If not, it prompts the user to add the default url and updates the application
// if they accept.
func promptDefaultURLs(ctx context.Context, cli *cli, client *management.Client, qsType string) error {
	if !strings.EqualFold(qsType, QSSpa) && !strings.EqualFold(qsType, QSWebApp) {
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
	shouldUpdate := false
	if confirmed := prompt.Confirm(formatURLPrompt(qsType)); confirmed {
		a.Callbacks = append(a.Callbacks, _defaultURL)
		a.AllowedLogoutURLs = append(a.AllowedLogoutURLs, _defaultURL)
		a.WebOrigins = append(a.WebOrigins, _defaultURL)
		shouldUpdate = true
	}
	if shouldUpdate {
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
	if strings.EqualFold(qsType, QSSpa) {
		p.WriteString(", logout URLs, and web origins?")
	} else {
		p.WriteString(" and logout URLs?")
	}
	return fmt.Sprintf(p.String(), _defaultURL)
}
