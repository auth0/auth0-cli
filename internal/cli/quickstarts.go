package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"

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
	cmd.AddCommand(setupQuickstartCmd(cli))

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
  auth0 qs ls --json-compact
  auth0 qs ls --csv`,
		RunE: listQuickstarts(cli),
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

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
	switch v {
	case "native":
		return qsNative
	case "spa":
		return qsSpa
	case "regular_web":
		return qsWebApp
	case "non_interactive":
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

// Flags for the setup command.
var (
	setupApp = Flag{
		Name:     "App",
		LongForm: "app",
		Help:     "Create an Auth0 application (SPA, regular web, or native)",
	}
	setupName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Auth0 application",
	}
	setupType = Flag{
		Name:     "Type",
		LongForm: "type",
		Help:     "Application type: spa, regular, native, or m2m",
	}
	setupFramework = Flag{
		Name:     "Framework",
		LongForm: "framework",
		Help:     "Framework to configure (e.g., react, nextjs, vue, express)",
	}
	setupBuildTool = Flag{
		Name:     "Build Tool",
		LongForm: "build-tool",
		Help:     "Build tool used by the project (vite, webpack, cra, none)",
	}
	setupPort = Flag{
		Name:     "Port",
		LongForm: "port",
		Help:     "Local port the application runs on (default varies by framework, e.g. 3000, 5173)",
	}
	setupCallbackURL = Flag{
		Name:     "Callback URL",
		LongForm: "callback-url",
		Help:     "Override the allowed callback URL for the application",
	}
	setupLogoutURL = Flag{
		Name:     "Logout URL",
		LongForm: "logout-url",
		Help:     "Override the allowed logout URL for the application",
	}
	setupWebOriginURL = Flag{
		Name:     "Web Origin URL",
		LongForm: "web-origin-url",
		Help:     "Override the allowed web origin URL for the application",
	}
	setupAPI = Flag{
		Name:     "API",
		LongForm: "api",
		Help:     "Create an Auth0 API resource server",
	}
	setupIdentifier = Flag{
		Name:        "Identifier",
		LongForm:    "identifier",
		Help:        "Unique URL identifier for the API (audience), e.g. https://my-api",
		AlsoKnownAs: []string{"audience"},
	}
	setupSigningAlg = Flag{
		Name:     "Signing Algorithm",
		LongForm: "signing-alg",
		Help:     "[API] Token signing algorithm: RS256, PS256, or HS256 (leave blank to be prompted interactively)",
	}
	setupScopes = Flag{
		Name:     "Scopes",
		LongForm: "scopes",
		Help:     "[API] Comma-separated list of permission scopes for the API",
	}
	setupTokenLifetime = Flag{
		Name:     "Token Lifetime",
		LongForm: "token-lifetime",
		Help:     "[API] Access token lifetime in seconds (default: 86400 = 24 hours)",
	}
	setupOfflineAccess = Flag{
		Name:     "Offline Access",
		LongForm: "offline-access",
		Help:     "Allow offline access (enables refresh tokens)",
	}
	setupLinkedAppID = Flag{
		Name:     "Linked App ID",
		LongForm: "linked-app-id",
		Help:     "[API] Client ID of an existing application to link to the API (skips app creation)",
	}
)

// SetupInputs holds the user-provided inputs for the setup command.
type SetupInputs struct {
	Name          string
	App           bool
	Type          string
	Framework     string
	BuildTool     string
	Port          int
	BundleID      string // Package/bundle ID for native apps, populated from detection.
	CallbackURL   string
	LogoutURL     string
	WebOriginURL  string
	API           bool
	Identifier    string
	SigningAlg    string
	Scopes        string
	TokenLifetime string
	OfflineAccess bool
	LinkedAppID   string // Client ID of an existing app to link to the API.
	MetaData      map[string]interface{}
}

func setupQuickstartCmd(cli *cli) *cobra.Command {
	var inputs SetupInputs

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Set up Auth0 for your quickstart application",
		Long: `Auto-detects your project, creates an Auth0 application and/or API, and generates a config file.

Workflows:
  --app                          Create an application (auto-detects framework).
  --api                          Create an API (prompts to create or link an app).
  --api --linked-app-id <id>     Create an API linked to an existing application.`,
		Example: `  # Interactive setup:
  auth0 quickstarts setup

  # App only:
  auth0 quickstarts setup --app --type spa --framework react

  # App with all options:
  auth0 quickstarts setup --app --type spa --framework react --build-tool vite --name "My SPA" --port 5173

  # API + new app:
  auth0 quickstarts setup --api --app --type regular --framework express --identifier https://my-api

  # API + existing app:
  auth0 quickstarts setup --api --linked-app-id <client-id> --identifier https://my-api

  # API with custom settings:
  auth0 quickstarts setup --api --linked-app-id <client-id> --identifier https://my-api --scopes "read:data,write:data"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupQuickstart(cmd, cli, &inputs)
		},
	}

	// App flags.
	registerSetupFlags(cmd, &inputs)

	cmd.MarkFlagsMutuallyExclusive("app", "linked-app-id")

	return cmd
}

// registerSetupFlags registers all flags for the setup command.
func registerSetupFlags(cmd *cobra.Command, inputs *SetupInputs) {
	// App flags.
	setupApp.RegisterBool(cmd, &inputs.App, false)
	setupName.RegisterString(cmd, &inputs.Name, "")
	setupType.RegisterString(cmd, &inputs.Type, "")
	setupFramework.RegisterString(cmd, &inputs.Framework, "")
	setupBuildTool.RegisterString(cmd, &inputs.BuildTool, "none")
	setupPort.RegisterInt(cmd, &inputs.Port, 0)
	setupCallbackURL.RegisterString(cmd, &inputs.CallbackURL, "")
	setupLogoutURL.RegisterString(cmd, &inputs.LogoutURL, "")
	setupWebOriginURL.RegisterString(cmd, &inputs.WebOriginURL, "")

	// API flags.
	setupAPI.RegisterBool(cmd, &inputs.API, false)
	setupIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	setupSigningAlg.RegisterString(cmd, &inputs.SigningAlg, "")
	setupScopes.RegisterString(cmd, &inputs.Scopes, "")
	setupTokenLifetime.RegisterString(cmd, &inputs.TokenLifetime, "")
	setupOfflineAccess.RegisterBool(cmd, &inputs.OfflineAccess, false)
	setupLinkedAppID.RegisterString(cmd, &inputs.LinkedAppID, "")
}

// runSetupQuickstart is the orchestration entry point for `quickstarts setup`.
func runSetupQuickstart(cmd *cobra.Command, cli *cli, inputs *SetupInputs) error {
	ctx := cmd.Context()

	if err := cli.setupWithAuthentication(ctx); err != nil {
		return fmt.Errorf("authentication required: %w", err)
	}

	// Validate that --linked-app-id is only used with --api.
	if inputs.LinkedAppID != "" && !inputs.API {
		return fmt.Errorf("--linked-app-id requires --api")
	}

	canPromptFlag := canPrompt(cmd)

	// -- Step 1: Decide what to create (App / API) --.
	if err := resolveSetupTargets(inputs, canPromptFlag); err != nil {
		return err
	}

	// -- Step 2: API flow — decide whether to create a new app or link an existing one --.
	if err := resolveAPIAppLink(cmd, cli, inputs, canPromptFlag); err != nil {
		return err
	}

	// -- Step 3: Auto-detect project framework --.
	if err := handleProjectDetection(cmd, cli, inputs, canPromptFlag); err != nil {
		return err
	}

	// -- Step 4: Resolve remaining prompts (type, framework, build tool) --.
	if err := validateNonInteractiveRequirements(*inputs, canPromptFlag); err != nil {
		return err
	}

	qsConfigKey, wasAutoSelected, err := getQuickstartConfigKey(cmd, inputs)
	if err != nil {
		return fmt.Errorf("failed to get quickstart configuration: %w", err)
	}
	if inputs.App && wasAutoSelected {
		cli.renderer.Infof("Auto-selected build tool %q for %s/%s", inputs.BuildTool, inputs.Type, inputs.Framework)
	}

	// -- Step 5: Collect application/API name --.
	if err = collectName(cmd, inputs); err != nil {
		return err
	}

	// -- Step 6: Prompt for port if not explicitly set --.
	if inputs.App && inputs.Type != "native" && inputs.Type != "m2m" {
		portStr := strconv.Itoa(inputs.Port)
		if err := setupPort.AskInt(cmd, &inputs.Port, &portStr); err != nil {
			return fmt.Errorf("failed to enter port: %w", err)
		}
		if inputs.Port < 1024 || inputs.Port > 65535 {
			return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", inputs.Port)
		}
	}

	// -- Step 7: Collect API-specific inputs (identifier, signing alg, token lifetime) --.
	if inputs.API {
		if err := collectAPIInputs(cmd, cli, inputs); err != nil {
			return err
		}
	}

	// -- Step 8: Create the Auth0 application client --.
	if inputs.App {
		clientID, err := createQuickstartApp(cmd, cli, *inputs, qsConfigKey)
		if err != nil {
			return err
		}
		// Store the new client ID so Step 9 can link it to the API via a client grant.
		inputs.LinkedAppID = clientID
	}

	// -- Step 9: Create the Auth0 API resource server --.
	if inputs.API {
		if err := createQuickstartAPI(ctx, cli, *inputs); err != nil {
			return err
		}
	}

	return nil
}

// resolveSetupTargets resolves the App/API selection, prompting when allowed.
func resolveSetupTargets(inputs *SetupInputs, canPromptFlag bool) error {
	if inputs.App || inputs.API {
		return nil
	}
	if !canPromptFlag {
		return fmt.Errorf("in --no-input mode, specify at least one of --app or --api")
	}

	const (
		optApp = "App Only"
		optAPI = "API (Associate with an App)"
	)

	var selection string
	q := prompt.SelectInput(
		"setup-target",
		"What do you want to create?",
		"Select App to create an application, or API to create an API linked to an app.",
		[]string{optApp, optAPI},
		optApp,
		true,
	)
	if err := prompt.AskOne(q, &selection); err != nil {
		return fmt.Errorf("failed to select target resource: %w", err)
	}

	switch selection {
	case optApp:
		inputs.App = true
	case optAPI:
		inputs.API = true
	default:
		return fmt.Errorf("unexpected selection: %q", selection)
	}
	return nil
}

// resolveAPIAppLink handles the API flow's app-linking decision.
// It either sets inputs.LinkedAppID (via picker or flag) or flips inputs.App=true
// so the regular App-creation flow runs and the new client ID is assigned later.
func resolveAPIAppLink(
	cmd *cobra.Command,
	cli *cli,
	inputs *SetupInputs,
	canPromptFlag bool,
) error {
	if !inputs.API {
		return nil
	}

	// --app and --api both set: create a new app and link it.
	if inputs.App {
		return nil
	}

	// --linked-app-id flag already provided.
	if inputs.LinkedAppID != "" {
		return nil
	}

	if !canPromptFlag {
		return fmt.Errorf("in --no-input mode with --api, specify --app to create a new app or --linked-app-id <client-id> to link an existing one")
	}

	const (
		actionCreateNew = "Create a new app"
		actionLink      = "Link to an existing app"
	)

	var action string
	actionQ := prompt.SelectInput(
		"link-app-action",
		"How should this API be associated with an app?",
		"Create a new app to link with the API, or link to an existing one in the tenant.",
		[]string{actionCreateNew, actionLink},
		actionCreateNew,
		true,
	)
	if err := prompt.AskOne(actionQ, &action); err != nil {
		return fmt.Errorf("failed to select link action: %w", err)
	}

	if action == actionCreateNew {
		inputs.App = true
		return nil
	}

	// Link to an existing app via the standard app picker.
	appPicker := Argument{
		Name: "Application",
		Help: "Select an existing application to authorize for this API.",
	}
	return appPicker.Pick(
		cmd,
		&inputs.LinkedAppID,
		cli.appPickerOptions(management.Parameter("app_type", "native,spa,regular_web")),
	)
}

// collectName prompts for the application/API name if not already provided.
func collectName(cmd *cobra.Command, inputs *SetupInputs) error {
	if setupName.IsSet(cmd) {
		if inputs.Name == "" {
			return fmt.Errorf("application name cannot be empty")
		}
		return nil
	}

	switch {
	case inputs.App:
		defaultName := inputs.Name
		if defaultName == "" {
			defaultName = "My App"
		}
		inputs.Name = defaultName
		if err := setupName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
			return fmt.Errorf("failed to enter application name: %w", err)
		}
		if inputs.Name == "" {
			return fmt.Errorf("application name cannot be empty")
		}

	case inputs.API && inputs.Name == "":
		cwd, _ := os.Getwd()
		defaultName := filepath.Base(cwd)
		if defaultName == "" || defaultName == "." {
			defaultName = "my-api"
		}
		inputs.Name = defaultName
		if err := setupName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
			return fmt.Errorf("failed to enter API name: %w", err)
		}
	}
	return nil
}

func collectAPIInputs(cmd *cobra.Command, cli *cli, inputs *SetupInputs) error {
	// Identifier.
	if !setupIdentifier.IsSet(cmd) {
		defaultID := inputs.Identifier
		if defaultID == "" && inputs.Name != "" {
			slug := strings.ToLower(strings.ReplaceAll(inputs.Name, " ", "-"))
			defaultID = "https://" + slug
		}
		inputs.Identifier = defaultID
		if err := setupIdentifier.Ask(cmd, &inputs.Identifier, &defaultID); err != nil {
			return fmt.Errorf("failed to enter API identifier: %w", err)
		}
	}
	if inputs.Identifier == "" {
		return fmt.Errorf("API identifier cannot be empty: use --identifier flag")
	}
	if err := validateAPIIdentifier(inputs.Identifier); err != nil {
		return err
	}

	// Fail fast if the (possibly user-overridden) identifier is already taken — avoids creating an orphaned app.
	if _, err := cli.api.ResourceServer.Read(cmd.Context(), url.PathEscape(inputs.Identifier)); err == nil {
		return fmt.Errorf("an API with identifier %q already exists; use a different identifier or delete the existing API first", inputs.Identifier)
	}

	// Token lifetime.
	if inputs.TokenLifetime == "" {
		defaultLifetime := strconv.Itoa(apiDefaultTokenLifetime)
		inputs.TokenLifetime = defaultLifetime
		if err := setupTokenLifetime.Ask(cmd, &inputs.TokenLifetime, &defaultLifetime); err != nil {
			return fmt.Errorf("failed to enter token lifetime: %w", err)
		}
		if inputs.TokenLifetime == "" {
			cli.renderer.Warnf("Token lifetime left blank; using default %d seconds (24 hours)", apiDefaultTokenLifetime)
			inputs.TokenLifetime = strconv.Itoa(apiDefaultTokenLifetime)
		}
	}

	// Signing algorithm.
	if inputs.SigningAlg == "" {
		signingAlgs := []string{"RS256", "PS256", "HS256"}
		defaultAlg := "RS256"
		inputs.SigningAlg = defaultAlg
		if err := setupSigningAlg.Select(cmd, &inputs.SigningAlg, signingAlgs, &defaultAlg); err != nil {
			return fmt.Errorf("failed to select signing algorithm: %w", err)
		}
	}
	if alg := inputs.SigningAlg; alg != "RS256" && alg != "PS256" && alg != "HS256" {
		return fmt.Errorf("invalid signing algorithm %q: must be RS256, PS256, or HS256", alg)
	}

	return nil
}

// handleProjectDetection runs project auto-detection for app flows and reconciles
// the detected values with explicit flags.
func handleProjectDetection(cmd *cobra.Command, cli *cli, inputs *SetupInputs, canPromptFlag bool) error {
	if !inputs.App {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Default to working-directory basename; DetectProject may override below.
	if inputs.Name == "" {
		inputs.Name = filepath.Base(cwd)
	}

	switch {
	case inputs.Type == "m2m":
		return nil

	case setupType.IsSet(cmd) && setupFramework.IsSet(cmd):
		typeFramework := inputs.Type + ":" + inputs.Framework

		// Resolve build tool from the known supported values when not explicitly set.
		if !setupBuildTool.IsSet(cmd) {
			if tools := auth0.FrameworkSupportedBuildTools[typeFramework]; len(tools) == 1 {
				inputs.BuildTool = tools[0]
				cli.renderer.Warnf("Auto-selected build tool %q for %s/%s", inputs.BuildTool, inputs.Type, inputs.Framework)
			} else if len(tools) > 1 {
				if !canPromptFlag {
					inputs.BuildTool = tools[0]
					cli.renderer.Warnf("Multiple build tools available for %s/%s; auto-selected %q (use --build-tool to override)", inputs.Type, inputs.Framework, inputs.BuildTool)
				} else {
					if err := setupBuildTool.Select(cmd, &inputs.BuildTool, tools, &tools[0]); err != nil {
						return fmt.Errorf("failed to select build tool: %w", err)
					}
				}
			}
		}

		// Only do a disk scan if the bundle ID is still needed.
		if inputs.BundleID == "" && auth0.IsBundleIDRequired(typeFramework) {
			detection := DetectProject(cwd)
			if detection.BundleID != "" {
				inputs.BundleID = detection.BundleID
			}
		}
		return nil
	}

	detection := DetectProject(cwd)

	switch {
	case detection.Detected:
		noInputMode := !canPromptFlag
		if len(detection.AmbiguousFrameworks) > 1 {
			renderAmbiguousDetectionSummary(cli, detection)
			if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
				applyDetectionToInputs(inputs, detection)
				if inputs.Framework == "" {
					if noInputMode {
						inputs.Framework = detection.AmbiguousFrameworks[0]
					} else {
						if err := setupFramework.Select(cmd, &inputs.Framework, detection.AmbiguousFrameworks, auth0.String(detection.AmbiguousFrameworks[0])); err != nil {
							return fmt.Errorf("failed to select framework: %w", err)
						}
					}
				}
			}
		} else if detection.Framework != "" {
			renderClearDetectionSummary(cli, detection)
			if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
				applyDetectionToInputs(inputs, detection)
				if inputs.Framework == "" {
					inputs.Framework = detection.Framework
				}
			}
		}
	default:
		if !canPromptFlag && inputs.Type == "" {
			return fmt.Errorf(
				"auto-detection failed: unable to auto detect application. " +
					"In --no-input mode provide --type, --framework, and optionally --build-tool " +
					"(e.g. --type spa --framework react --build-tool vite)",
			)
		}
		cli.renderer.Warnf("auto-detection failed: unable to auto detect application")
	}
	return nil
}

// renderClearDetectionSummary prints the bullet-list summary for an unambiguous detection.
func renderClearDetectionSummary(cli *cli, detection DetectionResult) {
	titleCaser := cases.Title(language.English)
	frameworkDisplay := frameworkDisplayName(detection.Framework)
	if detection.BuildTool != "" && detection.BuildTool != "none" {
		frameworkDisplay += " - " + titleCaser.String(detection.BuildTool)
	}

	const labelWidth = 9
	cli.renderer.Output("Detected in current directory:")
	cli.renderer.Detailf("%-*s : %s", labelWidth, "Framework", frameworkDisplay)
	cli.renderer.Detailf("%-*s : %s", labelWidth, "App type", detectionFriendlyAppType(detection.Type))
}

// renderAmbiguousDetectionSummary prints the summary when multiple package.json deps matched.
// It lists the candidate frameworks so the user has context for the framework-selection
// prompt that follows.
func renderAmbiguousDetectionSummary(cli *cli, detection DetectionResult) {
	candidates := make([]string, len(detection.AmbiguousFrameworks))
	for i, f := range detection.AmbiguousFrameworks {
		candidates[i] = frameworkDisplayName(f)
	}

	const labelWidth = 19
	cli.renderer.Output("Detected in current directory:")
	cli.renderer.Detailf("%-*s : %s", labelWidth, "Possible frameworks", strings.Join(candidates, ", "))
	cli.renderer.Detailf("%-*s : %s", labelWidth, "App type", detectionFriendlyAppType(detection.Type))
}

// validateNonInteractiveRequirements enforces flag requirements that apply only when prompts are disabled.
func validateNonInteractiveRequirements(inputs SetupInputs, canPromptFlag bool) error {
	if canPromptFlag {
		return nil
	}
	if inputs.App && inputs.Type != "" && inputs.Type != "m2m" && inputs.Framework == "" {
		return fmt.Errorf(
			"--framework is required in non-interactive mode when --type is %s: "+
				"use --framework and optionally --build-tool flags "+
				"(e.g. --framework react --build-tool vite)",
			inputs.Type,
		)
	}
	return nil
}

// printClientDetails prints post-creation guidance for the new Auth0 application.
// The config-file outcome is reported by WriteQuickstartConfig itself (success
// message or aborted-with-content warning), so it is intentionally not echoed here.
func printClientDetails(cli *cli, client *management.Client) {
	cli.renderer.Successf("App %s has been created in the management console", ansi.Magenta(client.GetName()))
	cli.renderer.Newline()
	manageTenantURL := formatManageTenantURL(cli.Config.DefaultTenant, &cli.Config)

	settingsURL := fmt.Sprintf("%s%s", manageTenantURL, formatAppSettingsPath(client.GetClientID()))
	cli.renderer.Successf("You can manage your application %s(%s) from here:", client.GetName(), ansi.Magenta(client.GetClientID()))
	cli.renderer.Detailf(ansi.Magenta(settingsURL))
	cli.renderer.Newline()

	if len(client.GetCallbacks()) > 0 {
		cli.renderer.Successf("Callback URLs registered in Auth0 Dashboard: %s", ansi.Magenta(strings.Join(client.GetCallbacks(), ", ")))
		cli.renderer.Newline()
	}
	if len(client.GetAllowedLogoutURLs()) > 0 {
		cli.renderer.Successf("Logout URLs registered: %s", ansi.Magenta(strings.Join(client.GetAllowedLogoutURLs(), ", ")))
		cli.renderer.Newline()
	}
}

func printAPIDetails(cli *cli, rs *management.ResourceServer) {
	cli.renderer.Newline()
	cli.renderer.Successf("An API %s has been created and registered", ansi.Magenta(rs.GetName()))
	cli.renderer.Detailf("Identifier: %s", ansi.Magenta(rs.GetIdentifier()))
	cli.renderer.Newline()

	manageTenantURL := formatManageTenantURL(cli.Config.DefaultTenant, &cli.Config)
	settingsURL := fmt.Sprintf("%s%s", manageTenantURL, formatAPISettingsPath(rs.GetID()))

	cli.renderer.Successf("You can manage your API from here:")
	cli.renderer.Detailf("%s", ansi.Magenta(settingsURL))
}

// createQuickstartApp creates an Auth0 application client for the given quickstart config key,
// writes the env config file, and prints setup guidance. It returns the newly created client ID.
func createQuickstartApp(cmd *cobra.Command, cli *cli, inputs SetupInputs, qsConfigKey string) (string, error) {
	config, exists := auth0.QuickstartConfigs[qsConfigKey]
	if !exists {
		return "", fmt.Errorf("unsupported quickstart arguments: %s. Supported types: %v", qsConfigKey, getSupportedQuickstartTypes())
	}

	expoScheme := readExpoScheme(inputs.Framework)
	nativeBundleID := resolveNativeBundleID(inputs)

	// For dotnet-mobile/MAUI, register the custom URI scheme callback.
	if (inputs.Framework == "dotnet-mobile" || inputs.Framework == "maui") && nativeBundleID != "" {
		config.RequestParams.Callbacks = []string{nativeBundleID + "://callback"}
	}

	client, err := generateClient(inputs, config.RequestParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate client: %w", err)
	}

	if err := ansi.Waiting(func() error {
		return cli.api.Client.Create(cmd.Context(), client)
	}); err != nil {
		return "", fmt.Errorf("failed to create application: %w", err)
	}

	// Inject the audience variable when an API is also being created.
	envValues := config.EnvValues
	if inputs.API && inputs.Identifier != "" && config.AudienceVar != "" {
		envValues = make(map[string]string, len(config.EnvValues)+1)
		for k, v := range config.EnvValues {
			envValues[k] = v
		}
		envValues[config.AudienceVar] = inputs.Identifier
	}

	resolvedEnv, err := replaceDetectionSub(envValues, cli.tenant, client, inputs.Port)
	if err != nil {
		return "", err
	}

	if err := cli.WriteQuickstartConfig(cmd, resolvedEnv, &config.Strategy); err != nil {
		return "", fmt.Errorf("failed to generate config file: %w", err)
	}

	printClientDetails(cli, client)
	printNativeGuidance(cli, inputs.Framework, expoScheme, nativeBundleID)

	return client.GetClientID(), nil
}

// readExpoScheme reads the "expo.scheme" field from app.json in the working directory.
// Returns the raw string value, or empty if the framework is not expo or the file is missing.
func readExpoScheme(framework string) string {
	if framework != "expo" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(cwd, "app.json"))
	if err != nil {
		return ""
	}
	var obj struct {
		Expo struct {
			Scheme string `json:"scheme"`
		} `json:"expo"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	return obj.Expo.Scheme
}

// resolveNativeBundleID determines the bundle/package ID for native frameworks.
func resolveNativeBundleID(inputs SetupInputs) string {
	if inputs.BundleID != "" {
		return inputs.BundleID
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	switch inputs.Framework {
	case "flutter", "react-native":
		return readMobileBundleID(cwd)
	case "maui", "dotnet-mobile":
		if content, ok := findCsprojContent(cwd); ok {
			return readDotnetMobileBundleID(content)
		}
	case "ionic-angular", "ionic-react", "ionic-vue":
		return readCapacitorAppID(cwd)
	}
	return ""
}

// printNativeGuidance prints post-setup callback URL guidance for native frameworks.
func printNativeGuidance(cli *cli, framework, expoScheme, bundleID string) {
	switch framework {
	case "expo":
		if expoScheme != "" {
			cli.renderer.Infof("Note: exp://localhost:19000 is registered for Expo Go development.")
			cli.renderer.Infof("For EAS/production builds, add %s:// to Allowed Callback URLs in the Auth0 Dashboard.", expoScheme)
		} else {
			cli.renderer.Infof("Note: exp://localhost:19000 is for Expo Go development only.")
			cli.renderer.Infof("For EAS/production builds, add your custom scheme URI (e.g., myapp://) to Allowed Callback URLs in the Auth0 Dashboard.")
		}
	case "flutter", "react-native":
		if bundleID != "" {
			cli.renderer.Infof("Add these Allowed Callback URLs in the Auth0 Dashboard:")
			cli.renderer.Infof("  Android: %s://%s/android/%s/callback", bundleID, cli.tenant, bundleID)
			cli.renderer.Infof("  iOS:     %s://%s/ios/%s/callback", bundleID, cli.tenant, bundleID)
		}
	case "maui", "dotnet-mobile":
		if bundleID != "" {
			cli.renderer.Infof("Registered %s://callback as the Allowed Callback URL.", bundleID)
		}
	case "ionic-angular", "ionic-react", "ionic-vue":
		if bundleID != "" {
			cli.renderer.Infof("Capacitor app ID: %s", bundleID)
			cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
		} else {
			cli.renderer.Warnf("Could not read Capacitor app ID. Ensure capacitor.config.json or capacitor.config.ts is present in your project root.")
			cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
		}
	case "jhipster":
		cli.renderer.Infof("Refer to JHipster documentation to complete the setup: https://www.jhipster.tech/security/#auth0")
	}
}

// createQuickstartAPI creates an Auth0 API resource server and optionally links it to an
// existing application client via a client grant.
func createQuickstartAPI(ctx context.Context, cli *cli, inputs SetupInputs) error {
	// API name = "<app-name>-API", fallback to identifier.
	apiName := inputs.Identifier
	if inputs.Name != "" {
		apiName = inputs.Name + "-API"
	}

	tokenLifetime, tokenErr := strconv.Atoi(inputs.TokenLifetime)
	if tokenErr != nil || tokenLifetime <= 0 {
		if inputs.TokenLifetime != "" && inputs.TokenLifetime != strconv.Itoa(apiDefaultTokenLifetime) {
			cli.renderer.Warnf("Invalid token lifetime %q, using default %d seconds", inputs.TokenLifetime, apiDefaultTokenLifetime)
		}
		tokenLifetime = apiDefaultTokenLifetime
	}

	rs := &management.ResourceServer{
		Name:               &apiName,
		Identifier:         &inputs.Identifier,
		SigningAlgorithm:   &inputs.SigningAlg,
		TokenLifetime:      &tokenLifetime,
		AllowOfflineAccess: &inputs.OfflineAccess,
	}

	if inputs.Scopes != "" {
		var scopeList []string
		for _, s := range strings.Split(inputs.Scopes, ",") {
			if s = strings.TrimSpace(s); s != "" {
				scopeList = append(scopeList, s)
			}
		}
		if len(scopeList) > 0 {
			rs.Scopes = apiScopesFor(scopeList)
		}
	}

	if err := ansi.Waiting(func() error {
		return cli.api.ResourceServer.Create(ctx, rs)
	}); err != nil {
		return fmt.Errorf("failed to create API with name %q and identifier %q: %w", apiName, inputs.Identifier, err)
	}
	printAPIDetails(cli, rs)

	// Link the app to the API via a client grant if an app was selected/created.
	if inputs.LinkedAppID != "" {
		emptyScopes := []string{}
		grant := &management.ClientGrant{
			ClientID: &inputs.LinkedAppID,
			Audience: &inputs.Identifier,
			Scope:    &emptyScopes,
		}
		if grantErr := ansi.Waiting(func() error {
			return cli.api.ClientGrant.Create(ctx, grant)
		}); grantErr != nil {
			cli.renderer.Warnf("Failed to link application to API: %v", grantErr)
		}
	}

	return nil
}

func getSupportedQuickstartTypes() []string {
	var types []string
	for key := range auth0.QuickstartConfigs {
		types = append(types, key)
	}
	sort.Strings(types)
	return types
}

// frameworksForType returns the list of unique frameworks available for the given app type.
func frameworksForType(qsType string) []string {
	seen := make(map[string]bool)
	var frameworks []string
	for key := range auth0.QuickstartConfigs {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) >= 2 && parts[0] == qsType {
			fw := parts[1]
			if !seen[fw] {
				seen[fw] = true
				frameworks = append(frameworks, fw)
			}
		}
	}
	sort.Strings(frameworks)
	return frameworks
}

// getQuickstartConfigKey resolves remaining missing prompts for App and API creation
// and returns the config map key for the selected framework.
// App/API selection and project detection are handled by the caller before this is invoked.
func getQuickstartConfigKey(cmd *cobra.Command, inputs *SetupInputs) (string, bool, error) {
	if !inputs.App {
		return "", false, nil
	}

	wasAutoSelected, err := resolveSetupInputs(cmd, inputs)
	if err != nil {
		return "", false, err
	}

	if inputs.Type == "m2m" {
		return "m2m:none:none", false, nil
	}

	configKey := fmt.Sprintf("%s:%s:%s", inputs.Type, inputs.Framework, inputs.BuildTool)

	return configKey, wasAutoSelected, nil
}

// resolveSetupInputs validates and fills missing fields on inputs by prompting the user
// where needed. It returns the updated inputs, whether a build tool was auto-selected,
// and any error encountered.
func resolveSetupInputs(cmd *cobra.Command, inputs *SetupInputs) (bool, error) {
	// Validate --type if provided.
	validTypes := []string{"spa", "regular", "native", "m2m"}
	if inputs.Type != "" && !slices.Contains(validTypes, inputs.Type) {
		return false, fmt.Errorf(
			"invalid --type %q: must be one of %s",
			inputs.Type, strings.Join(validTypes, ", "),
		)
	}

	// Prompt for --type if not provided.
	if inputs.Type == "" {
		defaultType := "spa"
		if err := setupType.Select(cmd, &inputs.Type, validTypes, &defaultType); err != nil {
			return false, fmt.Errorf("failed to select application type: %w", err)
		}
	}

	// M2M apps have no framework, port, or callback URLs.
	if inputs.Type == "m2m" {
		return false, nil
	}

	// Prompt for --framework filtered to the selected type.
	if inputs.Framework == "" {
		frameworks := frameworksForType(inputs.Type)
		if len(frameworks) == 0 {
			return false, fmt.Errorf("no frameworks available for type %q", inputs.Type)
		}
		if err := setupFramework.Select(cmd, &inputs.Framework, frameworks, &frameworks[0]); err != nil {
			return false, fmt.Errorf("failed to select framework: %w", err)
		}
	}

	// Resolve port from framework default before prompting.
	// Port stays 0 for native apps (react-native, expo, flutter) - no port needed.
	if inputs.Port == 0 {
		inputs.Port = defaultPortForFramework(inputs.Framework)
	}

	// If no explicit build tool and the "none" variant doesn't exist, resolve the best
	// supported build tool from the pre-built FrameworkBuildTools map.
	wasAutoSelected := false
	if inputs.BuildTool == "" || inputs.BuildTool == "none" {
		if _, exists := auth0.QuickstartConfigs[fmt.Sprintf("%s:%s:none", inputs.Type, inputs.Framework)]; !exists {
			if tools := auth0.FrameworkBuildTools[inputs.Type+":"+inputs.Framework]; len(tools) > 0 {
				inputs.BuildTool = tools[0]
				wasAutoSelected = true
			} else {
				inputs.BuildTool = "none"
			}
		} else {
			inputs.BuildTool = "none"
		}
	}

	return wasAutoSelected, nil
}

// applyDetectionToInputs copies fields from a DetectionResult into inputs, skipping
// any field that was already explicitly set. The framework field is NOT copied here
// because the ambiguous-candidate path requires a prompt before it can be resolved.
func applyDetectionToInputs(inputs *SetupInputs, d DetectionResult) {
	if inputs.Type == "" {
		inputs.Type = d.Type
	}
	if inputs.BuildTool == "" || inputs.BuildTool == "none" {
		inputs.BuildTool = d.BuildTool
	}
	if inputs.BundleID == "" && d.BundleID != "" {
		inputs.BundleID = d.BundleID
	}
}

// frameworkDisplayName returns a human-friendly display name for a framework key.
// It handles cases where the internal key (e.g. "vanilla-python") differs from the
// name a developer would expect to see (e.g. "Flask").
func frameworkDisplayName(framework string) string {
	switch framework {
	case "vanilla-python":
		return "Flask"
	case "vanilla-go":
		return "Go"
	case "vanilla-php":
		return "PHP"
	case "vanilla-javascript":
		return "Vanilla JS"
	case "vanilla-java":
		return "Java"
	default:
		titleCaser := cases.Title(language.English)
		return titleCaser.String(framework)
	}
}

// defaultPortForFramework returns the conventional port for a given framework name.
func defaultPortForFramework(framework string) int {
	switch framework {
	case "react", "vue", "svelte", "sveltekit", "vanilla-javascript":
		return 5173 // Vite default.
	case "angular":
		return 4200
	case "flask", "vanilla-python":
		return 5000
	case "django":
		return 8000
	case "laravel":
		return 8000
	case "java-ee", "vanilla-java", "jhipster":
		return 8080
	case "aspnet-mvc", "aspnet-owin":
		return 5000
	default:
		return 3000
	}
}

// validateAPIIdentifier returns an error if identifier is not a valid http:// or https:// URL.
func validateAPIIdentifier(identifier string) error {
	u, err := url.ParseRequestURI(identifier)
	if err != nil {
		return fmt.Errorf("invalid API identifier %q: must be a valid URI (e.g. https://my-api)", identifier)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid API identifier %q: must include a scheme and host (e.g. https://my-api)", identifier)
	}
	return nil
}

func generateClient(input SetupInputs, reqParams auth0.RequestParams) (*management.Client, error) {
	if input.Name == "" {
		input.Name = "My App"
	}

	if input.MetaData == nil {
		input.MetaData = map[string]interface{}{
			"created_by": "quickstart-docs-manual-cli",
		}
	}

	resolved := resolveRequestParams(reqParams, input.Name, input.Port)

	// Override URL fields with explicit flag values when provided.
	if input.CallbackURL != "" {
		resolved.Callbacks = []string{input.CallbackURL}
	}
	if input.LogoutURL != "" {
		resolved.AllowedLogoutURLs = []string{input.LogoutURL}
	}
	if input.WebOriginURL != "" {
		resolved.WebOrigins = []string{input.WebOriginURL}
	}

	algorithm := "RS256"
	oidcConformant := true
	client := &management.Client{
		Name:              &input.Name,
		AppType:           &resolved.AppType,
		Callbacks:         &resolved.Callbacks,
		AllowedLogoutURLs: &resolved.AllowedLogoutURLs,
		OIDCConformant:    &oidcConformant,
		JWTConfiguration: &management.ClientJWTConfiguration{
			Algorithm: &algorithm,
		},
		ClientMetadata: &input.MetaData,
	}

	if len(resolved.WebOrigins) > 0 {
		client.WebOrigins = &resolved.WebOrigins
	}

	return client, nil
}

// resolveRequestParams replaces DetectionSub placeholders in RequestParams fields
// with actual values derived from the user inputs.
func resolveRequestParams(reqParams auth0.RequestParams, name string, port int) auth0.RequestParams {
	if port == 0 {
		port = 3000
	}
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	callbacks := make([]string, len(reqParams.Callbacks))
	copy(callbacks, reqParams.Callbacks)
	logoutURLs := make([]string, len(reqParams.AllowedLogoutURLs))
	copy(logoutURLs, reqParams.AllowedLogoutURLs)
	webOrigins := make([]string, len(reqParams.WebOrigins))
	copy(webOrigins, reqParams.WebOrigins)

	resolvedName := reqParams.Name
	if resolvedName == auth0.DetectionSub {
		resolvedName = name
	}
	callbackPath := "/callback"
	if reqParams.CallbackPath != "" {
		callbackPath = reqParams.CallbackPath
	}
	for i, cb := range callbacks {
		switch cb {
		case auth0.DetectionSub:
			callbacks[i] = baseURL + callbackPath
		case auth0.DetectionSubAsBase:
			callbacks[i] = baseURL
		}
	}
	for i, u := range logoutURLs {
		if u == auth0.DetectionSub {
			logoutURLs[i] = baseURL
		}
	}
	for i, u := range webOrigins {
		if u == auth0.DetectionSub {
			webOrigins[i] = baseURL
		}
	}

	return auth0.RequestParams{
		AppType:           reqParams.AppType,
		Callbacks:         callbacks,
		AllowedLogoutURLs: logoutURLs,
		WebOrigins:        webOrigins,
		Name:              resolvedName,
		CallbackPath:      reqParams.CallbackPath,
	}
}

func replaceDetectionSub(envValues map[string]string, tenantDomain string, client *management.Client, port int) (map[string]string, error) {
	if port == 0 {
		port = 3000
	}
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	updatedEnvValues := make(map[string]string)

	for key, value := range envValues {
		if value != auth0.DetectionSub && value != auth0.DetectionSubAsBase {
			updatedEnvValues[key] = value
			continue
		}
		resolved, err := resolveDetectionSubValue(key, tenantDomain, baseURL, client)
		if err != nil {
			return nil, err
		}
		updatedEnvValues[key] = resolved
	}

	return updatedEnvValues, nil
}

// resolveDetectionSubValue maps a single env key to its runtime value.
func resolveDetectionSubValue(key, tenantDomain, baseURL string, client *management.Client) (string, error) {
	switch key {
	case "VITE_AUTH0_DOMAIN", "AUTH0_DOMAIN", "domain", "NUXT_AUTH0_DOMAIN",
		"auth0.domain", "auth0/domain", "Auth0:Domain", "auth0:Domain", "auth0_domain",
		"EXPO_PUBLIC_AUTH0_DOMAIN", "com.auth0.domain",
		"com_auth0_domain", "Domain":
		return tenantDomain, nil

	// Express SDK specifically requires the https:// prefix.
	case "ISSUER_BASE_URL":
		return "https://" + tenantDomain, nil

	// Spring Boot okta issuer specifically requires https:// and a trailing slash.
	case "okta.oauth2.issuer":
		return "https://" + tenantDomain + "/", nil

	case "VITE_AUTH0_CLIENT_ID", "AUTH0_CLIENT_ID", "clientId", "NUXT_AUTH0_CLIENT_ID",
		"CLIENT_ID", "auth0.clientId", "auth0/clientId", "okta.oauth2.client-id", "Auth0:ClientId",
		"auth0:ClientId", "auth0_client_id", "EXPO_PUBLIC_AUTH0_CLIENT_ID", "com.auth0.clientId",
		"com_auth0_client_id", "ClientId":
		return client.GetClientID(), nil

	case "AUTH0_CLIENT_SECRET", "NUXT_AUTH0_CLIENT_SECRET", "auth0.clientSecret", "auth0/clientSecret",
		"okta.oauth2.client-secret", "Auth0:ClientSecret", "auth0:ClientSecret",
		"auth0_client_secret", "com.auth0.clientSecret":
		return client.GetClientSecret(), nil

	case "AUTH0_SECRET", "NUXT_AUTH0_SESSION_SECRET", "SESSION_SECRET",
		"SECRET", "AUTH0_SESSION_ENCRYPTION_KEY", "AUTH0_COOKIE_SECRET":
		secret, err := generateState(32)
		if err != nil {
			return "", fmt.Errorf("failed to generate secret for %s: %w", key, err)
		}
		return secret, nil

	case "APP_BASE_URL", "NUXT_AUTH0_APP_BASE_URL", "BASE_URL", "AUTH0_BASE_URL":
		return baseURL, nil

	case "AUTH0_REDIRECT_URI", "AUTH0_CALLBACK_URL":
		return baseURL + "/callback", nil

	case "SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI":
		return "https://" + tenantDomain + "/", nil

	case "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID":
		return client.GetClientID(), nil

	case "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET":
		return client.GetClientSecret(), nil

	case "JHIPSTER_SECURITY_OAUTH2_AUDIENCE":
		return "https://" + tenantDomain + "/api/v2/", nil

	default:
		return "", fmt.Errorf("unhandled placeholder for env key %q: add it to replaceDetectionSub", key)
	}
}

// buildNestedMap converts a flat map with dot-delimited keys into a nested map,
// e.g. {"okta.oauth2.issuer": "x"} -> {"okta": {"oauth2": {"issuer": "x"}}}.
func buildNestedMap(flat map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range flat {
		parts := strings.Split(key, ".")
		current := result
		for i, part := range parts {
			if i == len(parts)-1 {
				if _, alreadyMap := current[part].(map[string]interface{}); !alreadyMap {
					current[part] = value
				}
			} else {
				next, ok := current[part].(map[string]interface{})
				if !ok {
					next = make(map[string]interface{})
					current[part] = next
				}
				current = next
			}
		}
	}
	return result
}

// xmlEscape replaces XML/HTML special characters with their entity equivalents
// so that generated XML config files are well-formed even when values contain
// characters like &, <, >, " or '.
func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (c *cli) WriteQuickstartConfig(cmd *cobra.Command, resolvedEnv map[string]string, strategy *auth0.FileOutputStrategy) error {
	if strategy == nil {
		strategy = &auth0.FileOutputStrategy{Path: ".env", Format: "dotenv"}
	}

	dir := filepath.Dir(strategy.Path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory structure %s: %w", dir, err)
		}
	}

	var contentBuilder strings.Builder

	switch strategy.Format {
	case "dotenv":
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "%s=\"%s\"\n", key, resolvedEnv[key])
		}

	case "properties":
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "%s=%s\n", key, resolvedEnv[key])
		}

	case "yaml":
		// Produce nested YAML from dot-delimited keys (e.g. Spring Boot application.yml).
		nested := buildNestedMap(resolvedEnv)
		yamlBytes, err := yaml.Marshal(nested)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(yamlBytes)

	case "rails-yaml":
		// Rails config/auth0.yml wraps credentials under the "development" environment key.
		devSection := make(map[string]interface{}, len(resolvedEnv))
		for k, v := range resolvedEnv {
			devSection[k] = v
		}
		wrapped := map[string]interface{}{"development": devSection}
		yamlBytes, err := yaml.Marshal(wrapped)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(yamlBytes)

	case "ts":
		contentBuilder.WriteString("export const environment = {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "  %s: '%s',\n", key, strings.ReplaceAll(resolvedEnv[key], "'", "\\'"))
		}
		contentBuilder.WriteString("};\n")

	case "angular-ts":
		// Angular SPA environment.ts nests domain and clientId under an auth0 object,
		// matching the official Angular quickstart: environment.auth0.domain / .clientId.
		contentBuilder.WriteString("export const environment = {\n")
		contentBuilder.WriteString("  production: false,\n")
		contentBuilder.WriteString("  auth0: {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "    %s: '%s',\n", key, strings.ReplaceAll(resolvedEnv[key], "'", "\\'"))
		}
		contentBuilder.WriteString("  },\n")
		contentBuilder.WriteString("};\n")

	case "dart":
		contentBuilder.WriteString("const Map<String, String> authConfig = {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "  '%s': '%s',\n", strings.ReplaceAll(key, "'", "\\'"), strings.ReplaceAll(resolvedEnv[key], "'", "\\'"))
		}
		contentBuilder.WriteString("};\n")

	case "json":
		// C# appsettings.json expects nested JSON: {"Auth0": {"Domain": "...", "ClientId": "..."}}.
		auth0Section := make(map[string]string)
		for key, val := range resolvedEnv {
			if !strings.HasPrefix(key, "Auth0:") {
				return fmt.Errorf("json formatter: key %q is missing required \"Auth0:\" prefix", key)
			}
			cleanKey := strings.TrimPrefix(key, "Auth0:")
			auth0Section[cleanKey] = val
		}
		jsonBody := map[string]interface{}{"Auth0": auth0Section}
		jsonBytes, err := json.MarshalIndent(jsonBody, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(jsonBytes)

	case "xml":
		// ASP.NET OWIN Web.config.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		contentBuilder.WriteString("<configuration>\n")
		contentBuilder.WriteString("  <appSettings>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "    <add key=\"%s\" value=\"%s\" />\n", xmlEscape(key), xmlEscape(resolvedEnv[key]))
		}
		contentBuilder.WriteString("  </appSettings>\n")
		contentBuilder.WriteString("</configuration>\n")

	case "webxml":
		// Java servlet web.xml context-param entries (mvc-auth-commons).
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString("<context-param>\n")
			fmt.Fprintf(&contentBuilder, "  <param-name>%s</param-name>\n", xmlEscape(key))
			fmt.Fprintf(&contentBuilder, "  <param-value>%s</param-value>\n", xmlEscape(resolvedEnv[key]))
			contentBuilder.WriteString("</context-param>\n")
		}

	case "javaee-webxml":
		// Java EE web.xml JNDI env-entry elements (auth0-java-mvc-commons).
		// Values are looked up via InitialContext.lookup("auth0.domain") etc.
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString("<env-entry>\n")
			fmt.Fprintf(&contentBuilder, "  <env-entry-name>%s</env-entry-name>\n", xmlEscape(key))
			contentBuilder.WriteString("  <env-entry-type>java.lang.String</env-entry-type>\n")
			fmt.Fprintf(&contentBuilder, "  <env-entry-value>%s</env-entry-value>\n", xmlEscape(resolvedEnv[key]))
			contentBuilder.WriteString("</env-entry>\n")
		}

	case "android-strings":
		// Android res/values/strings.xml - Auth0 SDK reads credentials via string resources.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		contentBuilder.WriteString("<resources>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "    <string name=\"%s\">%s</string>\n", xmlEscape(key), xmlEscape(resolvedEnv[key]))
		}
		contentBuilder.WriteString("</resources>\n")

	case "plist":
		// IOS Auth0.plist - Auth0 Swift SDK reads ClientId and Domain from this plist.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		contentBuilder.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
		contentBuilder.WriteString("<plist version=\"1.0\">\n")
		contentBuilder.WriteString("<dict>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			fmt.Fprintf(&contentBuilder, "    <key>%s</key>\n", key)
			fmt.Fprintf(&contentBuilder, "    <string>%s</string>\n", xmlEscape(resolvedEnv[key]))
		}
		contentBuilder.WriteString("</dict>\n")
		contentBuilder.WriteString("</plist>\n")
	}

	message := fmt.Sprintf("Proceed to overwrite '%s' file?", ansi.Bold(strategy.Path))
	if shouldCancelOverwrite(c, cmd, strategy.Path, message) {
		c.renderer.Warnf("Aborted creating %s. Please create it manually using the following content:", ansi.Cyan(strategy.Path))
		c.renderer.Newline()

		const border = "─────────────────────────────────────────────────────────────"
		c.renderer.Detailf("%s", border)
		for _, line := range strings.Split(strings.TrimRight(contentBuilder.String(), "\n"), "\n") {
			c.renderer.Detailf("%s", line)
		}
		c.renderer.Detailf("%s", border)
		c.renderer.Newline()
	} else {
		if err := os.WriteFile(strategy.Path, []byte(contentBuilder.String()), 0600); err != nil {
			return fmt.Errorf("failed to write .env file: %w", err)
		}

		c.renderer.Newline()
		c.renderer.Successf("%s file created successfully with your Auth0 configuration", ansi.Cyan(strategy.Path))
	}

	return nil
}
