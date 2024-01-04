package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// QuickStart app types and defaults.
const (
	qsNative       = "native"
	qsSpa          = "spa"
	qsWebApp       = "webapp"
	qsBackend      = "backend"
	qsDefaultURL   = "http://localhost"
	qspDefaultPort = 3000
)

var (
	qsClientID = Argument{
		Name: "Client ID",
		Help: "Client Id of an Auth0 application.",
	}

	qsStack = Flag{
		Name:      "Stack",
		LongForm:  "stack",
		ShortForm: "s",
		Help: "Tech/language of the Quickstart sample to download. " +
			"You can use the 'auth0 quickstarts list' command to see all available tech stacks. ",
		IsRequired: true,
	}
)

type qsInputs struct {
	ClientID string
	Stack    string

	Client     *management.Client
	Quickstart auth0.Quickstart

	QsTypeForClient string
}

func quickstartsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quickstarts",
		Short:   "Quickstart support for getting bootstrapped",
		Long:    "Step-by-step guides to quickly integrate Auth0 into your application.",
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
		Example: `  auth0 quickstarts list
  auth0 quickstarts ls
  auth0 qs list
  auth0 qs ls
  auth0 qs ls --json
  auth0 qs ls --csv`,
		RunE: listQuickstarts(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "csv")

	return cmd
}

func downloadQuickstartCmd(cli *cli) *cobra.Command {
	var inputs qsInputs

	cmd := &cobra.Command{
		Use:   "download",
		Args:  cobra.MaximumNArgs(1),
		Short: "Download a Quickstart sample app for a specific tech stack",
		Long: "Download a Quickstart sample application for that’s already configured for your Auth0 application. " +
			"There are many different tech stacks available.",
		Example: `  auth0 quickstarts download
  auth0 quickstarts download <app-id>
  auth0 quickstarts download <app-id> --stack <stack>
  auth0 qs download <app-id> -s <stack>
  auth0 qs download <app-id> -s "Next.js"
  auth0 qs download <app-id> -s "Next.js" --force`,
		RunE: downloadQuickstart(cli, &inputs),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	qsStack.RegisterString(cmd, &inputs.Stack, "")

	return cmd
}

func listQuickstarts(cli *cli) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		quickstarts, err := auth0.GetQuickstarts(cmd.Context())
		if err != nil {
			return err
		}

		cli.renderer.QuickstartList(quickstarts)

		return nil
	}
}

func downloadQuickstart(cli *cli, inputs *qsInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := inputs.fromArgs(cmd, args, cli); err != nil {
			return fmt.Errorf("failed to parse command inputs: %w", err)
		}

		quickstartPath, pathExists, err := computeQuickstartPathFromClientName(inputs.Client.GetName())
		if err != nil {
			return fmt.Errorf("failed to compute the path where to download the quickstart sample: %w", err)
		}

		if pathExists && !cli.force {
			message := fmt.Sprintf(
				"%s %s already exists.\n Directory contents will be replaced. Are you sure you want to proceed? ",
				ansi.Yellow("WARNING:"),
				quickstartPath,
			)
			if confirmed := prompt.Confirm(message); !confirmed {
				return nil
			}
		}

		err = ansi.Waiting(func() error {
			return inputs.Quickstart.Download(cmd.Context(), quickstartPath, inputs.Client)
		})
		if err != nil {
			return fmt.Errorf("failed to download quickstart sample: %w", err)
		}

		cli.renderer.Infof("Quickstart sample successfully downloaded at %s", quickstartPath)

		if err := promptDefaultURLs(cmd.Context(), cli, inputs.Client, inputs.QsTypeForClient, inputs.Stack); err != nil {
			return err
		}

		qsSamplePath, err := inputs.Quickstart.SamplePath(quickstartPath)
		if err != nil {
			return err
		}

		readme, err := loadQuickstartSampleReadme(qsSamplePath)
		if err != nil {
			cli.renderer.Infof(inputs.Quickstart.DownloadInstructions)
		} else {
			cli.renderer.Markdown(readme)
		}

		relativeQSSamplePath, err := relativeQuickstartSamplePath(qsSamplePath)
		if err != nil {
			return err
		}

		cli.renderer.Infof("%s Start with `cd %s`", ansi.Faint("Hint:"), relativeQSSamplePath)

		return nil
	}
}

func computeQuickstartPathFromClientName(clientName string) (quickstartPath string, pathExists bool, err error) {
	currDir, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	re := regexp.MustCompile(`[^\w]+`)
	friendlyName := re.ReplaceAllString(clientName, "-")
	target := path.Join(currDir, friendlyName)

	pathExists = true
	if _, err := os.Stat(target); err != nil {
		if !os.IsNotExist(err) {
			return "", false, err
		}
		pathExists = false
	}

	const readWriteAndExecutePermission = os.FileMode(0755)
	if err := os.MkdirAll(target, readWriteAndExecutePermission); err != nil {
		return "", false, err
	}

	return target, pathExists, nil
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
	data, err := os.ReadFile(path.Join(samplePath, "README.md"))
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
func promptDefaultURLs(ctx context.Context, cli *cli, client *management.Client, qsType string, qsStack string) error {
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

	if !containsStr(client.GetCallbacks(), defaultCallbackURL) {
		callbacks := append(client.GetCallbacks(), defaultCallbackURL)
		a.Callbacks = &callbacks
	}

	if !containsStr(client.GetAllowedLogoutURLs(), defaultURL) {
		allowedLogoutURLs := append(a.GetAllowedLogoutURLs(), defaultURL)
		a.AllowedLogoutURLs = &allowedLogoutURLs
	}

	if strings.EqualFold(qsType, qsSpa) {
		if !containsStr(client.GetAllowedOrigins(), defaultURL) {
			allowedOrigins := append(a.GetAllowedOrigins(), defaultURL)
			a.AllowedOrigins = &allowedOrigins
		}

		if !containsStr(client.GetWebOrigins(), defaultURL) {
			webOrigins := append(a.GetWebOrigins(), defaultURL)
			a.WebOrigins = &webOrigins
		}
	}

	callbackURLChanged := len(client.GetCallbacks()) != len(a.GetCallbacks())
	otherURLsChanged := len(client.GetAllowedLogoutURLs()) != len(a.GetAllowedLogoutURLs()) ||
		len(client.GetAllowedOrigins()) != len(a.GetAllowedOrigins()) ||
		len(client.GetWebOrigins()) != len(a.GetWebOrigins())

	if !callbackURLChanged && !otherURLsChanged {
		return nil
	}

	if confirmed := prompt.Confirm(urlPromptFor(qsType, qsStack)); confirmed {
		err := ansi.Waiting(func() error {
			return cli.api.Client.Update(ctx, client.GetClientID(), a)
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

func (i *qsInputs) fromArgs(cmd *cobra.Command, args []string, cli *cli) error {
	if len(args) == 0 {
		if err := qsClientID.Pick(
			cmd,
			&i.ClientID,
			cli.appPickerOptions(management.Parameter("app_type", "native,spa,regular_web,non_interactive")),
		); err != nil {
			return err
		}
	} else {
		i.ClientID = args[0]
	}

	var client *management.Client
	err := ansi.Waiting(func() (err error) {
		client, err = cli.api.Client.Read(cmd.Context(), i.ClientID)
		return
	})
	if err != nil {
		return fmt.Errorf("failed to find client with ID %q, please verify your client ID: %w", i.ClientID, err)
	}

	i.Client = client
	i.QsTypeForClient = quickstartsTypeFor(client.GetAppType())

	var quickstarts auth0.Quickstarts
	err = ansi.Waiting(func() error {
		quickstarts, err = auth0.GetQuickstarts(cmd.Context())
		return err
	})
	if err != nil {
		return err
	}

	if i.Stack == "" {
		quickstartsByType, err := quickstarts.FilterByType(i.QsTypeForClient)
		if err != nil {
			return err
		}

		if err := qsStack.Select(cmd, &i.Stack, quickstartsByType.Stacks(), nil); err != nil {
			return err
		}
	}

	quickstart, err := quickstarts.FindByStack(i.Stack)
	if err != nil {
		return err
	}

	i.Quickstart = quickstart

	return nil
}
