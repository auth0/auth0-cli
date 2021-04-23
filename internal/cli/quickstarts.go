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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

// QuickStart app types and defaults
const (
	qsNative       = "native"
	qsSpa          = "spa"
	qsWebApp       = "webapp"
	qsBackend      = "backend"
	qsDefaultURL   = "http://localhost"
	qspDefaultPort = 3000
)

const (
	quickstartEndpoint           = `https://auth0.com/docs/package/v2`
	quickstartContentType        = `application/json`
	quickstartOrg                = "auth0-samples"
	quickstartDefaultCallbackURL = `https://YOUR_APP/callback`
)

var (
	//go:embed data/quickstarts.json
	qsBuf             []byte
	quickstartsByType = func() (qs map[string][]auth0.Quickstart) {
		if err := json.Unmarshal(qsBuf, &qs); err != nil {
			panic(err)
		}
		return
	}()

	qsClientID = Argument{
		Name: "Client ID",
		Help: "Client Id of an Auth0 application.",
	}

	qsStack = Flag{
		Name:       "Stack",
		LongForm:   "stack",
		ShortForm:  "s",
		Help:       "Tech/Language of the quickstart sample to download.",
		IsRequired: true,
	}
)

func quickstartsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quickstarts",
		Short:   "Quickstart support for getting bootstrapped",
		Long:    "Quickstart support for getting bootstrapped.",
		Aliases: []string{"qs"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listQuickstartsCmd(cli))
	cmd.AddCommand(downloadQuickstartCmd(cli))

	return cmd
}

func listQuickstartsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List the available Quickstarts",
		Long:    "List the available Quickstarts.",
		Example: `auth0 quickstarts list
auth0 quickstarts ls
auth0 qs list
auth0 qs ls`,
		Run: func(cmd *cobra.Command, args []string) {
			cli.renderer.QuickstartList(quickstartsByType)
		},
	}

	return cmd
}

func downloadQuickstartCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ClientID string
		Stack    string
	}

	cmd := &cobra.Command{
		Use:     "download",
		Args:    cobra.MaximumNArgs(1),
		Short:   "Download a Quickstart sample app for a specific tech stack",
		Long:    "Download a Quickstart sample app for a specific tech stack.",
		Example: `auth0 quickstarts download --stack <stack>
auth0 qs download --stack <stack>`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !canPrompt(cmd) {
				return errors.New("This command can only be run on interactive mode")
			}

			if len(args) == 0 {
				err := qsClientID.Pick(cmd, &inputs.ClientID, cli.appPickerOptions)
				if err != nil {
					return err
				}
			} else {
				inputs.ClientID = args[0]
			}

			var client *management.Client
			err := ansi.Waiting(func() error {
				var err error
				client, err = cli.api.Client.Read(inputs.ClientID)
				return err
			})

			if err != nil {
				return fmt.Errorf("An unexpected error occurred, please verify your Client Id: %v", err.Error())
			}

			if inputs.Stack == "" {
				// get the valid types for this App:
				stacks, err := quickstartStacksFromType(client.GetAppType())
				if err != nil {
					return fmt.Errorf("An unexpected error occurred: %v", err)
				}
				// ask for input using the valid types only:
				if err := qsStack.Select(cmd, &inputs.Stack, stacks, nil); err != nil {
					return err
				}
			}

			target, exists, err := quickstartPathFor(client)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred: %v", err)
			}

			if exists && !cli.force {
				if confirmed := prompt.Confirm(fmt.Sprintf("WARNING: %s already exists.\n Are you sure you want to proceed?", target)); !confirmed {
					return nil
				}
			}

			q, err := getQuickstart(client.GetAppType(), inputs.Stack)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred with the specified stack %v: %v", inputs.Stack, err)
			}

			err = ansi.Waiting(func() error {
				return downloadQuickStart(cmd.Context(), cli, client, target, q)
			})

			if err != nil {
				return fmt.Errorf("Unable to download quickstart sample: %v", err)
			}

			cli.renderer.Infof("Quickstart sample sucessfully downloaded at %s", target)

			qsType := quickstartsTypeFor(client.GetAppType())
			if err := promptDefaultURLs(cli, client, qsType, inputs.Stack); err != nil {
				return err
			}

			qsSamplePath := path.Join(target, q.Samples[0])
			readme, err := loadQuickstartSampleReadme(qsSamplePath) // Some QS have non-markdown READMEs (eg auth0-python uses rst)

			if err == nil {
				cli.renderer.Markdown(readme)
			} else {
				cli.renderer.Infof("%s You might wanna check out the Quickstart sample README", ansi.Faint("Hint:"))
			}

			relativeQSSamplePath, err := relativeQuickstartSamplePath(qsSamplePath)
			if err != nil {
				return err
			}

			cli.renderer.Infof("%s Start with 'cd %s'", ansi.Faint("Hint:"), relativeQSSamplePath)

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	qsStack.RegisterString(cmd, &inputs.Stack, "")
	return cmd
}

func downloadQuickStart(ctx context.Context, cli *cli, client *management.Client, target string, q auth0.Quickstart) error {
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
		return unexpectedError(err)
	}

	req, err := http.NewRequest("POST", quickstartEndpoint, buf)
	if err != nil {
		return unexpectedError(err)
	}
	req.Header.Set("Content-Type", quickstartContentType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return unexpectedError(err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	tmpfile, err := ioutil.TempFile("", "auth0-quickstart*.zip")
	if err != nil {
		return unexpectedError(err)
	}

	_, err = io.Copy(tmpfile, res.Body)
	if err != nil {
		return unexpectedError(err)
	}

	if err := tmpfile.Close(); err != nil {
		return unexpectedError(err)
	}
	defer os.Remove(tmpfile.Name())

	if err := os.RemoveAll(target); err != nil {
		return unexpectedError(err)
	}

	if err := archiver.Unarchive(tmpfile.Name(), target); err != nil {
		return unexpectedError(err)
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

func getQuickstart(t, stack string) (auth0.Quickstart, error) {
	qsType := quickstartsTypeFor(t)
	quickstarts, ok := quickstartsByType[qsType]
	if !ok {
		return auth0.Quickstart{}, fmt.Errorf("Unknown quickstart type: %s", qsType)
	}
	for _, q := range quickstarts {
		if strings.EqualFold(q.Name, stack) {
			return q, nil
		}
	}
	return auth0.Quickstart{}, fmt.Errorf("Quickstart not found for %s/%s", qsType, stack)
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

func loadQuickstartSampleReadme(samplePath string) (string, error) {
	data, err := ioutil.ReadFile(path.Join(samplePath, "README.md"))
	if err != nil {
		return "", unexpectedError(err)
	}

	return string(data), nil
}

func relativeQuickstartSamplePath(samplePath string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", unexpectedError(err)
	}

	relativePath, err := filepath.Rel(dir, samplePath)
	if err != nil {
		return "", unexpectedError(err)
	}

	return relativePath, nil
}

// promptDefaultURLs checks whether the application is SPA or WebApp and
// whether the app has already added the default quickstart url to allowed url lists.
// If not, it prompts the user to add the default url and updates the application
// if they accept.
func promptDefaultURLs(cli *cli, client *management.Client, qsType string, qsStack string) error {
	defaultURL := defaultURLFor(qsStack)
	defaultCallbackURL := defaultCallbackURLFor(qsStack)

	if !strings.EqualFold(qsType, qsSpa) && !strings.EqualFold(qsType, qsWebApp) {
		return nil
	}

	a := &management.Client{
		Callbacks:         client.Callbacks,
		AllowedLogoutURLs: client.AllowedLogoutURLs,
		AllowedOrigins:    client.AllowedOrigins,
		WebOrigins:        client.WebOrigins,
	}

	if !containsStr(client.Callbacks, defaultCallbackURL) {
		a.Callbacks = append(a.Callbacks, defaultCallbackURL)
	}

	if !containsStr(client.AllowedLogoutURLs, defaultURL) {
		a.AllowedLogoutURLs = append(a.AllowedLogoutURLs, defaultURL)
	}

	if strings.EqualFold(qsType, qsSpa) {
		if !containsStr(client.AllowedOrigins, defaultURL) {
			a.AllowedOrigins = append(a.AllowedOrigins, defaultURL)
		}

		if !containsStr(client.WebOrigins, defaultURL) {
			a.WebOrigins = append(a.WebOrigins, defaultURL)
		}
	}

	callbackURLChanged := len(client.Callbacks) != len(a.Callbacks)
	otherURLsChanged := len(client.AllowedLogoutURLs) != len(a.AllowedLogoutURLs) ||
		len(client.AllowedOrigins) != len(a.AllowedOrigins) ||
		len(client.WebOrigins) != len(a.WebOrigins)

	if !callbackURLChanged && !otherURLsChanged {
		return nil
	}

	if confirmed := prompt.Confirm(urlPromptFor(qsType, qsStack)); confirmed {
		err := ansi.Waiting(func() error {
			return cli.api.Client.Update(client.GetClientID(), a)
		})
		if err != nil {
			return err
		}
		cli.renderer.Infof("Application successfully updated")
	}
	return nil
}

// urlPromptFor creates the correct prompt based on app type for
// asking the user if they would like to add default urls.
func urlPromptFor(qsType string, qsStack string) string {
	var p strings.Builder
	p.WriteString("Quickstarts use localhost, do you want to add %s to the list\n of allowed callback URLs")
	switch strings.ToLower(qsStack) {
	case "next.js": // See https://github.com/auth0/auth0-cli/issues/200
		p.WriteString(" and %s to the list of allowed logout URLs?")
		return fmt.Sprintf(p.String(), defaultCallbackURLFor(qsStack), defaultURLFor(qsStack))
	default:
		if strings.EqualFold(qsType, qsSpa) {
			p.WriteString(", logout URLs, origins and web origins?")
		} else {
			p.WriteString(" and logout URLs?")
		}
	}
	return fmt.Sprintf(p.String(), defaultURLFor(qsStack))
}

func defaultURLFor(s string) string {
	switch strings.ToLower(s) {
	case "angular": // See https://github.com/auth0-samples/auth0-angular-samples/issues/225#issuecomment-806448893
		return defaultURL(qsDefaultURL, 4200)
	default:
		return defaultURL(qsDefaultURL, qspDefaultPort)
	}
}

func defaultCallbackURLFor(s string) string {
	switch strings.ToLower(s) {
	case "next.js": // See https://github.com/auth0/auth0-cli/issues/200
		return fmt.Sprintf("%s/api/auth/callback", defaultURLFor(s))
	default:
		return defaultURLFor(s)
	}
}

func defaultURL(url string, port int) string {
	return fmt.Sprintf("%s:%d", url, port)
}
