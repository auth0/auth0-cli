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
	cmd.AddCommand(setupQuickstartCmdExperimental(cli))

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

var (
	qsType = Flag{
		Name:       "Type",
		LongForm:   "type",
		ShortForm:  "t",
		Help:       "Type of quickstart (vite, nextjs, fastify, jhipster-rwa)",
		IsRequired: true,
	}
	qsAppName = Flag{
		Name:      "Name",
		LongForm:  "name",
		ShortForm: "n",
		Help:      "Name of the Auth0 application (default: 'My App' for vite, nextjs and fastify, 'JHipster' for jhipster-rwa)",
	}
	qsPort = Flag{
		Name:      "Port",
		LongForm:  "port",
		ShortForm: "p",
		Help:      "Port number for the application (default: 5173 for vite, 3000 for nextjs/fastify, 8080 for jhipster-rwa)",
	}
)

// Flags for the setup-experimental command.
var (
	setupExpApp = Flag{
		Name:     "App",
		LongForm: "app",
		Help:     "Create an Auth0 application (SPA, regular web, or native)",
	}
	setupExpName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Auth0 application",
	}
	setupExpType = Flag{
		Name:     "Type",
		LongForm: "type",
		Help:     "Application type: spa, regular, native, or m2m",
	}
	setupExpFramework = Flag{
		Name:     "Framework",
		LongForm: "framework",
		Help:     "Framework to configure (e.g., react, nextjs, vue, express)",
	}
	setupExpBuildTool = Flag{
		Name:     "Build Tool",
		LongForm: "build-tool",
		Help:     "Build tool used by the project (vite, webpack, cra, none)",
	}
	setupExpPort = Flag{
		Name:     "Port",
		LongForm: "port",
		Help:     "Local port the application runs on (default varies by framework, e.g. 3000, 5173)",
	}
	setupExpCallbackURL = Flag{
		Name:     "Callback URL",
		LongForm: "callback-url",
		Help:     "Override the allowed callback URL for the application",
	}
	setupExpLogoutURL = Flag{
		Name:     "Logout URL",
		LongForm: "logout-url",
		Help:     "Override the allowed logout URL for the application",
	}
	setupExpWebOriginURL = Flag{
		Name:     "Web Origin URL",
		LongForm: "web-origin-url",
		Help:     "Override the allowed web origin URL for the application",
	}
	setupExpAPI = Flag{
		Name:     "API",
		LongForm: "api",
		Help:     "Create an Auth0 API resource server",
	}
	setupExpIdentifier = Flag{
		Name:        "Identifier",
		LongForm:    "identifier",
		Help:        "Unique URL identifier for the API (audience), e.g. https://my-api",
		AlsoKnownAs: []string{"audience"},
	}
	setupExpSigningAlg = Flag{
		Name:     "Signing Algorithm",
		LongForm: "signing-alg",
		Help:     "[API] Token signing algorithm: RS256, PS256, or HS256 (leave blank to be prompted interactively)",
	}
	setupExpScopes = Flag{
		Name:     "Scopes",
		LongForm: "scopes",
		Help:     "[API] Comma-separated list of permission scopes for the API",
	}
	setupExpTokenLifetime = Flag{
		Name:     "Token Lifetime",
		LongForm: "token-lifetime",
		Help:     "[API] Access token lifetime in seconds (default: 86400 = 24 hours)",
	}
	setupExpOfflineAccess = Flag{
		Name:     "Offline Access",
		LongForm: "offline-access",
		Help:     "Allow offline access (enables refresh tokens)",
	}
)

// SetupInputs holds the user-provided inputs for the setup-experimental command.
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
	MetaData      map[string]interface{}
}

func setupQuickstartCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Type string
		Name string
		Port int
	}

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Set up Auth0 for your quickstart application",
		Long: "Creates an Auth0 application and generates a .env file with the necessary configuration.\n\n" +
			"The command will:\n" +
			"  1. Check if you are authenticated (and prompt for login if needed)\n" +
			"  2. Create an Auth0 application based on the specified type\n" +
			"  3. Generate a .env file with the appropriate environment variables\n\n" +
			"Supported types:\n" +
			"  - vite: For client-side SPAs (React, Vue, Svelte, etc.)\n" +
			"  - nextjs: For Next.js server-side applications\n" +
			"  - fastify: For Fastify web applications\n" +
			"  - jhipster-rwa: For JHipster regular web applications",
		Example: `  auth0 quickstarts setup --type vite
  auth0 quickstarts setup --type nextjs
  auth0 quickstarts setup --type fastify
  auth0 quickstarts setup --type vite --name "My App"
  auth0 quickstarts setup --type nextjs --port 8080
  auth0 quickstarts setup --type jhipster-rwa
  auth0 qs setup --type fastify -n "My App" -p 3000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if inputs.Type != "" {
				normalizedType := strings.ToLower(inputs.Type)
				if normalizedType != "vite" && normalizedType != "nextjs" && normalizedType != "fastify" && normalizedType != "jhipster-rwa" {
					return fmt.Errorf("unsupported quickstart type: %s (supported types: vite, nextjs, fastify, jhipster-rwa)", inputs.Type)
				}
			}

			if err := qsType.Select(cmd, &inputs.Type, []string{"vite", "nextjs", "fastify", "jhipster-rwa"}, nil); err != nil {
				return err
			}

			if err := cli.setupWithAuthentication(ctx); err != nil {
				return fmt.Errorf("authentication required: %w", err)
			}

			defaultName := "My App"
			var defaultPort int

			switch inputs.Type {
			case "vite":
				defaultPort = 5173
			case "nextjs", "fastify":
				defaultPort = 3000
			case "jhipster-rwa":
				defaultName = "JHipster"
				defaultPort = 8080
			}

			// If name is not explicitly set (is empty), ask for it or use default.
			if inputs.Name == "" {
				inputs.Name = defaultName
				if err := qsAppName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
					return err
				}
			}

			// If port is not explicitly set (is 0), ask for it or use default.
			if inputs.Port == 0 {
				inputs.Port = defaultPort
				defaultPortStr := strconv.Itoa(defaultPort)
				if err := qsPort.Ask(cmd, &inputs.Port, &defaultPortStr); err != nil {
					return err
				}
			}

			if inputs.Port < 1024 || inputs.Port > 65535 {
				return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", inputs.Port)
			}

			baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)
			appType := appTypeRegularWeb
			var callbacks, logoutURLs, origins, webOrigins []string

			// Configure URLs based on app type.
			switch inputs.Type {
			case "vite":
				appType = appTypeSPA
				callbacks = []string{baseURL}
				logoutURLs = []string{baseURL}
				origins = []string{baseURL}
				webOrigins = []string{baseURL}
			case "nextjs":
				callbackURL := fmt.Sprintf("%s/api/auth/callback", baseURL)
				callbacks = []string{callbackURL}
				logoutURLs = []string{baseURL}
			case "fastify":
				callbackURL := fmt.Sprintf("%s/auth/callback", baseURL)
				callbacks = []string{callbackURL}
				logoutURLs = []string{baseURL}
			case "jhipster-rwa":
				callbackURL := fmt.Sprintf("%s/login/oauth2/code/oidc", baseURL)
				callbacks = []string{callbackURL}
				logoutURLs = []string{baseURL}
			}

			cli.renderer.Infof("Creating Auth0 application '%s'...", inputs.Name)

			oidcConformant := true
			algorithm := "RS256"
			metadata := map[string]interface{}{
				"created_by": "quickstart-docs-manual-cli",
			}

			a := &management.Client{
				Name:              &inputs.Name,
				AppType:           &appType,
				Callbacks:         &callbacks,
				AllowedLogoutURLs: &logoutURLs,
				OIDCConformant:    &oidcConformant,
				JWTConfiguration: &management.ClientJWTConfiguration{
					Algorithm: &algorithm,
				},
				ClientMetadata: &metadata,
			}

			if inputs.Type == "vite" {
				a.AllowedOrigins = &origins
				a.WebOrigins = &webOrigins
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Client.Create(ctx, a)
			}); err != nil {
				return fmt.Errorf("failed to create application: %w", err)
			}

			cli.renderer.Infof("Application created successfully with Client ID: %s", a.GetClientID())

			tenant, err := cli.Config.GetTenant(cli.tenant)
			if err != nil {
				return fmt.Errorf("failed to get tenant: %w", err)
			}

			envFileName := ".env"
			var envContent strings.Builder

			switch inputs.Type {
			case "vite":
				fmt.Fprintf(&envContent, "VITE_AUTH0_DOMAIN=%s\n", tenant.Domain)
				fmt.Fprintf(&envContent, "VITE_AUTH0_CLIENT_ID=%s\n", a.GetClientID())

			case "nextjs":
				secret, err := generateState(32)
				if err != nil {
					return fmt.Errorf("failed to generate AUTH0_SECRET: %w", err)
				}

				fmt.Fprintf(&envContent, "AUTH0_DOMAIN=%s\n", tenant.Domain)
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_ID=%s\n", a.GetClientID())
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_SECRET=%s\n", a.GetClientSecret())
				fmt.Fprintf(&envContent, "AUTH0_SECRET=%s\n", secret)
				fmt.Fprintf(&envContent, "APP_BASE_URL=%s\n", baseURL)

			case "fastify":
				sessionSecret, err := generateState(64)
				if err != nil {
					return fmt.Errorf("failed to generate SESSION_SECRET: %w", err)
				}

				fmt.Fprintf(&envContent, "AUTH0_DOMAIN=%s\n", tenant.Domain)
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_ID=%s\n", a.GetClientID())
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_SECRET=%s\n", a.GetClientSecret())
				fmt.Fprintf(&envContent, "SESSION_SECRET=%s\n", sessionSecret)
				fmt.Fprintf(&envContent, "APP_BASE_URL=%s\n", baseURL)

			case "jhipster-rwa":
				envFileName = ".auth0.env"
				fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI=https://%s/\n", tenant.Domain)
				fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID=%s\n", a.GetClientID())
				fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET=%s\n", a.GetClientSecret())
				fmt.Fprintf(&envContent, "JHIPSTER_SECURITY_OAUTH2_AUDIENCE=https://%s/api/v2/\n", tenant.Domain)
			}

			message := fmt.Sprintf("     Proceed to overwrite '%s' file? : ", envFileName)
			if shouldCancelOverwrite(cli, cmd, envFileName, message) {
				cli.renderer.Warnf("Aborted creating %s file. Please create it manually using the following content:\n\n"+
					"─────────────────────────────────────────────────────────────\n"+"%s"+
					"─────────────────────────────────────────────────────────────\n", envFileName, envContent.String())
			} else {
				if err = os.WriteFile(envFileName, []byte(envContent.String()), 0600); err != nil {
					return fmt.Errorf("failed to write .env file: %w", err)
				}

				cli.renderer.Infof("%s file created successfully with your Auth0 configuration\n", envFileName)
			}

			switch inputs.Type {
			case "jhipster-rwa":
				cli.renderer.Infof("Please refer to the JHipster documentation https://www.jhipster.tech/security/#auth0 to complete the setup")
			default:
				cli.renderer.Infof("Next steps: \n"+
					"       1. Install dependencies: npm install \n"+
					"       2. Start your application: npm run dev\n"+
					"       3. Open your browser at %s", baseURL)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	qsType.RegisterString(cmd, &inputs.Type, "")
	qsAppName.RegisterString(cmd, &inputs.Name, "")
	qsPort.RegisterInt(cmd, &inputs.Port, 0)

	return cmd
}

func setupQuickstartCmdExperimental(cli *cli) *cobra.Command {
	var inputs SetupInputs

	cmd := &cobra.Command{
		Use:   "setup-experimental",
		Args:  cobra.NoArgs,
		Short: "Set up Auth0 for your quickstart application",
		Long: "Creates an Auth0 application and/or API and generates a config file with the necessary Auth0 settings.\n\n" +
			"The command will:\n" +
			"  1. Check if you are authenticated (and prompt for login if needed)\n" +
			"  2. Auto-detect your project framework from the current directory\n" +
			"  3. Create an Auth0 application and/or API resource server\n" +
			"  4. Generate a config file with the appropriate environment variables\n\n" +
			"Supported frameworks are dynamically loaded from the QuickstartConfigs map.",
		Example: `  auth0 quickstarts setup-experimental
  auth0 quickstarts setup-experimental --app --framework react --type spa
  auth0 quickstarts setup-experimental --api --identifier https://my-api
  auth0 quickstarts setup-experimental --app --api --name "My App"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := cli.setupWithAuthentication(ctx); err != nil {
				return fmt.Errorf("authentication required: %w", err)
			}

			// LinkedAppClientID tracks which app client ID to link to the API
			// (either a newly created app or one selected from the tenant).
			var linkedAppClientID string
			canPromptFlag := canPrompt(cmd)

			// -- Step 1: Decide what to create (App / API / both) --.
			if !inputs.App && !inputs.API {
				if !canPromptFlag {
					return fmt.Errorf("in --no-input mode, specify at least one of --app or --api")
				}
				var selections []string
				if err := prompt.AskMultiSelect(
					"What do you want to create? (select whatever applies)",
					&selections,
					"App", "API",
				); err != nil {
					return fmt.Errorf("failed to select target resource(s): %w", err)
				}
				for _, s := range selections {
					switch strings.ToLower(s) {
					case "app":
						inputs.App = true
					case "api":
						inputs.API = true
					}
				}
				if !inputs.App && !inputs.API {
					return fmt.Errorf("please select at least one option: App and/or API")
				}
			}

			// -- Step 2: Auto-detect project framework --.
			if inputs.App {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}

				// M2M apps have no framework or port; skip DetectProject entirely.
				if inputs.Type == "m2m" {
					if inputs.Name == "" {
						inputs.Name = filepath.Base(cwd)
					}
				} else {
					detection := DetectProject(cwd)

					typeFromFlag := setupExpType.IsSet(cmd)
					frameworkFromFlag := setupExpFramework.IsSet(cmd)

					switch {
					case typeFromFlag && frameworkFromFlag:
						// User explicitly specified type and framework via flags; skip detection UI.
						if inputs.Name == "" {
							inputs.Name = detection.AppName
						}
						// If build tool was not explicitly provided, read it from detected config
						// files (e.g. vite.config.ts) rather than defaulting to "none" statically.
						if !setupExpBuildTool.IsSet(cmd) && detection.BuildTool != "" {
							inputs.BuildTool = detection.BuildTool
						}
						if inputs.BundleID == "" && detection.BundleID != "" {
							inputs.BundleID = detection.BundleID
						}
					case detection.Detected:
						noInputMode := !canPromptFlag
						if len(detection.AmbiguousFrameworks) > 1 {
							// Multiple package.json deps matched - show partial summary and ask user to disambiguate.
							cli.renderer.InfofBullet("Detected in current directory")
							cli.renderer.InfofBullet("Framework: %s", "Could not be determined")
							cli.renderer.InfofBullet("App type: %s", detectionFriendlyAppType(detection.Type))
							if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
								inputs = applyDetectionToInputs(inputs, detection)
								if inputs.Framework == "" {
									if noInputMode {
										inputs.Framework = detection.AmbiguousFrameworks[0]
									} else {
										defaultFramework := detection.AmbiguousFrameworks[0]
										if err := setupExpFramework.Select(cmd, &inputs.Framework, detection.AmbiguousFrameworks, &defaultFramework); err != nil {
											return fmt.Errorf("failed to select framework: %w", err)
										}
									}
								}
							}
						} else if detection.Framework != "" {
							// Single clear detection - show summary and confirm.
							titleCaser := cases.Title(language.English)
							frameworkDisplay := frameworkDisplayName(detection.Framework)
							if detection.BuildTool != "" && detection.BuildTool != "none" {
								frameworkDisplay += " - " + titleCaser.String(detection.BuildTool)
							}
							cli.renderer.InfofBullet("Detected in current directory")
							cli.renderer.InfofBullet("Framework: %s", frameworkDisplay)
							cli.renderer.InfofBullet("App type: %s", detectionFriendlyAppType(detection.Type))
							cli.renderer.InfofBullet("App name: %s", detection.AppName)
							if detection.Port > 0 {
								cli.renderer.InfofBullet("Port: %d", detection.Port)
							}

							if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
								inputs = applyDetectionToInputs(inputs, detection)
								if inputs.Framework == "" {
									inputs.Framework = detection.Framework
								}
							}
						}
					default:
						// No detection signal found - notify the user and pre-fill name from directory.
						if !canPromptFlag && inputs.Type == "" {
							return fmt.Errorf(
								"auto-detection failed: unable to auto detect application. " +
									"In --no-input mode provide --type, --framework, and optionally --build-tool " +
									"(e.g. --type spa --framework react --build-tool vite)",
							)
						}
						cli.renderer.Warnf("auto-detection failed: unable to auto detect application")
						if inputs.Name == "" {
							inputs.Name = detection.AppName
						}
					}
				}
			}

			// -- Step 3: Resolve remaining prompts for App / API --
			// In non-interactive mode, --type alone is not enough; --framework is also required.
			if !canPromptFlag && inputs.App && inputs.Type != "" && inputs.Type != "m2m" && inputs.Framework == "" {
				return fmt.Errorf(
					"--framework is required in non-interactive mode when --type is %s: "+
						"use --framework and optionally --build-tool flags "+
						"(e.g. --framework react --build-tool vite)",
					inputs.Type,
				)
			}
			qsConfigKey, updatedInputs, wasAutoSelected, err := getQuickstartConfigKey(cmd, inputs)
			if err != nil {
				return fmt.Errorf("failed to get quickstart configuration: %w", err)
			}
			inputs = updatedInputs
			if inputs.App && wasAutoSelected {
				cli.renderer.Infof("Auto-selected build tool %q for %s/%s (no exact match for 'none')", inputs.BuildTool, inputs.Type, inputs.Framework)
			}

			// -- Step 3b: Collect application name --.
			if inputs.App {
				if !setupExpName.IsSet(cmd) {
					defaultName := inputs.Name
					if defaultName == "" {
						defaultName = "My App"
					}
					inputs.Name = defaultName
					if err := setupExpName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
						return fmt.Errorf("failed to enter application name: %w", err)
					}
					if inputs.Name == "" {
						return fmt.Errorf("application name cannot be empty")
					}
				}
				if inputs.Name == "" {
					return fmt.Errorf("application name cannot be empty")
				}
			}

			// -- Step 3d: Prompt for port if not explicitly set --.
			if inputs.App && inputs.Type != "native" && inputs.Type != "m2m" {
				portStr := strconv.Itoa(inputs.Port)
				if err := setupExpPort.AskInt(cmd, &inputs.Port, &portStr); err != nil {
					return fmt.Errorf("failed to enter port: %w", err)
				}
				if inputs.Port < 1024 || inputs.Port > 65535 {
					return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", inputs.Port)
				}
			}

			// -- Step 3c: Collect API name for API-only flow --.
			if inputs.API && !inputs.App {
				// Collect API name if not already set (pre-fill from CWD folder name).
				if inputs.Name == "" && !setupExpName.IsSet(cmd) {
					cwd, _ := os.Getwd()
					defaultName := filepath.Base(cwd)
					if defaultName == "" || defaultName == "." {
						defaultName = "my-api"
					}
					inputs.Name = defaultName
					if err := setupExpName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
						return fmt.Errorf("failed to enter application name: %w", err)
					}
				}
			}

			if inputs.API {
				// Prompt for the identifier if not explicitly provided via flag.
				if !setupExpIdentifier.IsSet(cmd) {
					// Compute a suggested default without pre-populating inputs.Identifier.
					defaultID := inputs.Identifier
					if defaultID == "" && inputs.Name != "" {
						slug := strings.ToLower(strings.ReplaceAll(inputs.Name, " ", "-"))
						defaultID = "https://" + slug
					}
					inputs.Identifier = defaultID
					if err := setupExpIdentifier.Ask(cmd, &inputs.Identifier, &defaultID); err != nil {
						return fmt.Errorf("failed to enter API identifier: %w", err)
					}
				}

				if inputs.Identifier == "" {
					return fmt.Errorf("API identifier cannot be empty: use --identifier flag")
				}

				if err := validateAPIIdentifier(inputs.Identifier); err != nil {
					return err
				}

				// If the flag was not set, prompt interactively; fall back to 86400 in non-interactive mode.
				if inputs.TokenLifetime == "" {
					defaultLifetime := "86400"
					inputs.TokenLifetime = defaultLifetime
					if err := setupExpTokenLifetime.Ask(cmd, &inputs.TokenLifetime, &defaultLifetime); err != nil {
						return fmt.Errorf("failed to enter token lifetime: %w", err)
					}
					if inputs.TokenLifetime == "" {
						cli.renderer.Warnf("Token lifetime left blank; using default 86400 seconds (24 hours)")
						inputs.TokenLifetime = defaultLifetime
					}
				}

				if inputs.SigningAlg == "" {
					signingAlgs := []string{"RS256", "PS256", "HS256"}
					defaultAlg := "RS256"
					inputs.SigningAlg = defaultAlg
					if err := setupExpSigningAlg.Select(cmd, &inputs.SigningAlg, signingAlgs, &defaultAlg); err != nil {
						return fmt.Errorf("failed to select signing algorithm: %w", err)
					}
				}

				if alg := inputs.SigningAlg; alg != "RS256" && alg != "PS256" && alg != "HS256" {
					return fmt.Errorf("invalid signing algorithm %q: must be RS256, PS256, or HS256", alg)
				}

				// For API-only: fetch existing apps and let the user select one to link.
				if !inputs.App {
					var appList *management.ClientList
					var appListErr error
					_ = ansi.Waiting(func() error {
						appList, appListErr = cli.api.Client.List(
							ctx,
							management.Parameter("app_type", "native,spa,regular_web"),
							management.Parameter("is_global", "false"),
						)
						return appListErr
					})
					if appListErr != nil {
						cli.renderer.Warnf("Could not fetch existing applications: %v. You can link the API to an app manually.", appListErr)
					}

					appOptions := []string{"Skip"}
					appIDByName := make(map[string]string)
					if appList != nil && len(appList.Clients) > 0 {
						named := make([]string, 0, len(appList.Clients))
						for _, c := range appList.Clients {
							name := c.GetName()
							named = append(named, name)
							appIDByName[name] = c.GetClientID()
						}
						named = append(named, "Skip")
						appOptions = named
					}

					if canPromptFlag {
						var selectedAppName string
						q := prompt.SelectInput(
							"link-app",
							"Select App to register API",
							"Select an existing application to authorize for this API, or skip",
							appOptions,
							appOptions[0],
							true,
						)
						if err := prompt.AskOne(q, &selectedAppName); err != nil {
							return fmt.Errorf("failed to select app: %w", err)
						}
						if selectedAppName != "Skip" {
							linkedAppClientID = appIDByName[selectedAppName]
						}
					}
				}
			}

			// -- Step 4: Create the Auth0 application client --.
			if inputs.App {
				clientID, err := createQuickstartApp(ctx, cli, inputs, qsConfigKey)
				if err != nil {
					return err
				}
				linkedAppClientID = clientID
			}

			// -- Step 5: Create the Auth0 API resource server --.
			if inputs.API {
				if err := createQuickstartAPI(ctx, cli, inputs, linkedAppClientID); err != nil {
					return err
				}
			}

			return nil
		},
	}

	// App flags.
	setupExpApp.RegisterBool(cmd, &inputs.App, false)
	setupExpName.RegisterString(cmd, &inputs.Name, "")
	setupExpType.RegisterString(cmd, &inputs.Type, "")
	setupExpFramework.RegisterString(cmd, &inputs.Framework, "")
	setupExpBuildTool.RegisterString(cmd, &inputs.BuildTool, "none")
	setupExpPort.RegisterInt(cmd, &inputs.Port, 0)
	setupExpCallbackURL.RegisterString(cmd, &inputs.CallbackURL, "")
	setupExpLogoutURL.RegisterString(cmd, &inputs.LogoutURL, "")
	setupExpWebOriginURL.RegisterString(cmd, &inputs.WebOriginURL, "")

	// API flags.
	setupExpAPI.RegisterBool(cmd, &inputs.API, false)
	setupExpIdentifier.RegisterString(cmd, &inputs.Identifier, "")
	setupExpSigningAlg.RegisterString(cmd, &inputs.SigningAlg, "")
	setupExpScopes.RegisterString(cmd, &inputs.Scopes, "")
	setupExpTokenLifetime.RegisterString(cmd, &inputs.TokenLifetime, "")
	setupExpOfflineAccess.RegisterBool(cmd, &inputs.OfflineAccess, false)

	return cmd
}

func printClientDetails(cli *cli, client *management.Client, port int, configFileLocation string) {
	cli.renderer.Successf("An application %q has been created in the management console", client.GetName())
	cli.renderer.Detailf("Client ID: %s", ansi.Magenta(client.GetClientID()))
	cli.renderer.Newline()

	cli.renderer.Successf("You can manage your application from here:")
	cli.renderer.Detailf("%s", ansi.Magenta(fmt.Sprintf("https://manage.auth0.com/dashboard/#/applications/%s/settings", client.GetClientID())))
	cli.renderer.Newline()

	if client.Callbacks != nil && len(client.GetCallbacks()) > 0 {
		cli.renderer.Successf("Callback URLs registered in Auth0 Dashboard:")
		cli.renderer.Detailf("%s", ansi.Magenta(strings.Join(client.GetCallbacks(), ", ")))
		cli.renderer.Newline()
	}
	if client.AllowedLogoutURLs != nil && len(client.GetAllowedLogoutURLs()) > 0 {
		cli.renderer.Successf("Logout URLs registered:")
		cli.renderer.Detailf("%s", ansi.Magenta(strings.Join(client.GetAllowedLogoutURLs(), ", ")))
		cli.renderer.Newline()
	}
	cli.renderer.Successf("Config file created: %s", ansi.Magenta(configFileLocation))
}

func printAPIDetails(cli *cli, rs *management.ResourceServer) {
	cli.renderer.Successf("An API %q has been created and registered", rs.GetName())
	cli.renderer.Detailf("Identifier: %s", ansi.Magenta(rs.GetIdentifier()))
	cli.renderer.Newline()
	cli.renderer.Successf("You can manage your API from here:")
	cli.renderer.Detailf("%s", ansi.Magenta(fmt.Sprintf("https://manage.auth0.com/dashboard/#/apis/%s/settings", rs.GetID())))
}

// createQuickstartApp creates an Auth0 application client for the given quickstart config key,
// writes the env config file, and prints setup guidance. It returns the newly created client ID.
func createQuickstartApp(ctx context.Context, cli *cli, inputs SetupInputs, qsConfigKey string) (string, error) {
	config, exists := auth0.QuickstartConfigs[qsConfigKey]
	if !exists {
		return "", fmt.Errorf("unsupported quickstart arguments: %s. Supported types: %v", qsConfigKey, getSupportedQuickstartTypes())
	}

	// For Expo, read the production URI scheme from app.json (expo.scheme).
	// Custom schemes like "myapp://" are not registered automatically because
	// Auth0 API rejects bare custom-scheme URIs (no host component). Instead,
	// the scheme is surfaced in post-setup guidance so the user can add it manually.
	var expoScheme string
	if inputs.Framework == "expo" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			expoScheme = readExpoScheme(cwd)
			if expoScheme == "" {
				// Warn when app.json has a scheme that is not a valid RFC 3986 URI scheme.
				if raw := readRawExpoScheme(cwd); raw != "" {
					cli.renderer.Warnf("app.json expo.scheme %q is not a valid URI scheme (must start with a letter and contain only letters, digits, +, -, .); scheme will be ignored.", raw)
				}
			}
		}
	}

	// Resolve the bundle/package ID for native app guidance output.
	// The callback URL includes the Auth0 domain, so it can only be constructed after
	// the tenant config is fetched below.
	// Prefer the BundleID already populated by DetectProject to avoid re-reading disk.
	var nativeBundleID string
	switch {
	case inputs.BundleID != "":
		nativeBundleID = inputs.BundleID
	case inputs.Framework == "flutter" || inputs.Framework == "react-native":
		// Fallback for when framework was specified via --framework flag (detection not run).
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			nativeBundleID = readMobileBundleID(cwd)
		}
	case inputs.Framework == "maui" || inputs.Framework == "dotnet-mobile":
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			if csprojContent, ok := findCsprojContent(cwd); ok {
				nativeBundleID = readDotnetMobileBundleID(csprojContent)
			}
		}
	case inputs.Framework == "ionic-angular" || inputs.Framework == "ionic-react" || inputs.Framework == "ionic-vue":
		// Fallback for when framework was specified via --framework flag (detection not run).
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			nativeBundleID = readCapacitorAppID(cwd)
		}
	}

	// For dotnet-mobile and MAUI, the custom URI scheme callback is derived from the
	// ApplicationId in the .csproj. Register it in Auth0 when the bundle ID is known
	// so the developer does not need a manual dashboard update.
	if (inputs.Framework == "dotnet-mobile" || inputs.Framework == "maui") && nativeBundleID != "" {
		config.RequestParams.Callbacks = []string{nativeBundleID + "://callback"}
	}

	client, err := generateClient(inputs, config.RequestParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate client: %w", err)
	}

	if err := ansi.Waiting(func() error {
		return cli.api.Client.Create(ctx, client)
	}); err != nil {
		return "", fmt.Errorf("failed to create application: %w", err)
	}

	// When an API is also being created, inject the audience variable so the
	// config file contains the API identifier the app should request tokens for.
	envValues := config.EnvValues
	if inputs.API && inputs.Identifier != "" && config.AudienceVar != "" {
		envValues = make(map[string]string, len(config.EnvValues)+1)
		for k, v := range config.EnvValues {
			envValues[k] = v
		}
		envValues[config.AudienceVar] = inputs.Identifier
	}

	envFilePath, err := GenerateAndWriteQuickstartConfig(&config.Strategy, envValues, cli.tenant, client, inputs.Port)
	if err != nil {
		return "", fmt.Errorf("failed to generate config file: %w", err)
	}
	printClientDetails(cli, client, inputs.Port, filepath.Base(envFilePath))

	// Post-setup guidance for Expo: exp://localhost:19000 only covers Expo Go.
	// Inform the user about EAS/production build requirements.
	if inputs.Framework == "expo" {
		if expoScheme != "" {
			cli.renderer.Infof("Note: exp://localhost:19000 is registered for Expo Go development.")
			cli.renderer.Infof("For EAS/production builds, add %s:// to Allowed Callback URLs in the Auth0 Dashboard.", expoScheme)
		} else {
			cli.renderer.Infof("Note: exp://localhost:19000 is for Expo Go development only.")
			cli.renderer.Infof("For EAS/production builds, add your custom scheme URI (e.g., myapp://) to Allowed Callback URLs in the Auth0 Dashboard.")
		}
	}

	// Post-setup guidance for Flutter and .NET Mobile apps: show the
	// callback URLs to register in the Auth0 Dashboard. These use the
	// app's bundle/package ID and the tenant domain, both of which are
	// now available.
	switch inputs.Framework {
	case "flutter", "react-native":
		if nativeBundleID != "" {
			// The bundle ID is used directly as the URI scheme. RFC 3986 permits
			// hyphens in URI schemes, and both iOS CFBundleURLSchemes and Android
			// intent filters support them natively.
			cli.renderer.Infof("Add these Allowed Callback URLs in the Auth0 Dashboard:")
			cli.renderer.Infof("  Android: %s://%s/android/%s/callback", nativeBundleID, cli.tenant, nativeBundleID)
			cli.renderer.Infof("  iOS:     %s://%s/ios/%s/callback", nativeBundleID, cli.tenant, nativeBundleID)
		}
	case "maui", "dotnet-mobile":
		if nativeBundleID != "" {
			cli.renderer.Infof("Registered %s://callback as the Allowed Callback URL.", nativeBundleID)
		}
	case "ionic-angular", "ionic-react", "ionic-vue":
		if nativeBundleID != "" {
			// Capacitor intercepts http://localhost in the WebView (already registered).
			// Surface the appId so the user can configure deep links if needed.
			cli.renderer.Infof("Capacitor app ID: %s", nativeBundleID)
			cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
		} else {
			// No Capacitor config found - remind the user where it should be.
			cli.renderer.Warnf("Could not read Capacitor app ID. Ensure capacitor.config.json or capacitor.config.ts is present in your project root.")
			cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
		}
	case "jhipster":
		cli.renderer.Infof("Refer to JHipster documentation to complete the setup: https://www.jhipster.tech/security/#auth0")
	}

	return client.GetClientID(), nil
}

// createQuickstartAPI creates an Auth0 API resource server and optionally links it to an
// existing application client via a client grant.
func createQuickstartAPI(ctx context.Context, cli *cli, inputs SetupInputs, linkedAppClientID string) error {
	// API name = "<app-name>-API", fallback to identifier.
	apiName := inputs.Identifier
	if inputs.Name != "" {
		apiName = inputs.Name + "-API"
	}

	cli.renderer.Infof("Creating API resource server %q with identifier %q...", apiName, inputs.Identifier)
	tokenLifetime, tokenErr := strconv.Atoi(inputs.TokenLifetime)
	if tokenErr != nil || tokenLifetime <= 0 {
		if inputs.TokenLifetime != "" && inputs.TokenLifetime != "86400" {
			cli.renderer.Warnf("Invalid token lifetime %q, using default 86400 seconds", inputs.TokenLifetime)
		}
		tokenLifetime = 86400
	}

	rs := &management.ResourceServer{
		Name:             &apiName,
		Identifier:       &inputs.Identifier,
		SigningAlgorithm: &inputs.SigningAlg,
		TokenLifetime:    &tokenLifetime,
	}
	if inputs.OfflineAccess {
		allow := true
		rs.AllowOfflineAccess = &allow
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
		return fmt.Errorf("failed to create API: %w", err)
	}
	printAPIDetails(cli, rs)

	// Link the app to the API via a client grant if an app was selected/created.
	if linkedAppClientID != "" {
		emptyScopes := []string{}
		grant := &management.ClientGrant{
			ClientID: &linkedAppClientID,
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
func getQuickstartConfigKey(cmd *cobra.Command, inputs SetupInputs) (string, SetupInputs, bool, error) {
	if !inputs.App {
		return "", inputs, false, nil
	}

	inputs, wasAutoSelected, err := resolveSetupInputs(cmd, inputs)
	if err != nil {
		return "", inputs, false, err
	}

	if inputs.Type == "m2m" {
		return "m2m:none:none", inputs, false, nil
	}

	configKey := fmt.Sprintf("%s:%s:%s", inputs.Type, inputs.Framework, inputs.BuildTool)

	return configKey, inputs, wasAutoSelected, nil
}

// resolveSetupInputs validates and fills missing fields on inputs by prompting the user
// where needed. It returns the updated inputs, whether a build tool was auto-selected,
// and any error encountered.
func resolveSetupInputs(cmd *cobra.Command, inputs SetupInputs) (SetupInputs, bool, error) {
	// Validate --type if provided.
	validTypes := []string{"spa", "regular", "native", "m2m"}
	if inputs.Type != "" && !slices.Contains(validTypes, inputs.Type) {
		return inputs, false, fmt.Errorf(
			"invalid --type %q: must be one of %s",
			inputs.Type, strings.Join(validTypes, ", "),
		)
	}

	// Prompt for --type if not provided.
	if inputs.Type == "" {
		defaultType := "spa"
		if err := setupExpType.Select(cmd, &inputs.Type, validTypes, &defaultType); err != nil {
			return inputs, false, fmt.Errorf("failed to select application type: %w", err)
		}
	}

	// M2M apps have no framework, port, or callback URLs.
	if inputs.Type == "m2m" {
		return inputs, false, nil
	}

	// Prompt for --framework filtered to the selected type.
	if inputs.Framework == "" {
		frameworks := frameworksForType(inputs.Type)
		if len(frameworks) == 0 {
			return inputs, false, fmt.Errorf("no frameworks available for type %q", inputs.Type)
		}
		if err := setupExpFramework.Select(cmd, &inputs.Framework, frameworks, &frameworks[0]); err != nil {
			return inputs, false, fmt.Errorf("failed to select framework: %w", err)
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

	return inputs, wasAutoSelected, nil
}

// applyDetectionToInputs copies fields from a DetectionResult into inputs, skipping
// any field that was already explicitly set. The framework field is NOT copied here
// because the ambiguous-candidate path requires a prompt before it can be resolved.
func applyDetectionToInputs(inputs SetupInputs, d DetectionResult) SetupInputs {
	if inputs.Type == "" {
		inputs.Type = d.Type
	}
	if inputs.BuildTool == "" || inputs.BuildTool == "none" {
		inputs.BuildTool = d.BuildTool
	}
	if inputs.Port == 0 {
		inputs.Port = d.Port
	}
	if inputs.Name == "" {
		inputs.Name = d.AppName
	}
	if inputs.BundleID == "" && d.BundleID != "" {
		inputs.BundleID = d.BundleID
	}
	return inputs
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
	case "jhipster":
		return "JHipster"
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
	case "spring-boot", "java-ee", "vanilla-java", "jhipster":
		return 8080
	default:
		return 3000
	}
}

// validateAPIIdentifier returns an error if identifier is not a valid http:// or https:// URL.
func validateAPIIdentifier(identifier string) error {
	// ParseRequestURI is stricter than Parse: it rejects relative URLs, fragments,
	// and empty strings. The host check still catches bare schemes like "http://"
	// that ParseRequestURI accepts without error.
	_, err := url.ParseRequestURI(identifier)
	if err == nil || len(identifier) != 24 {
		return nil
	}
	return fmt.Errorf("invalid API identifier %q: must be a valid URL beginning with http:// or https://", identifier)
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

// GenerateAndWriteQuickstartConfig takes the selected stack, resolves the dynamic values,
// and writes them to the appropriate file in the Current Working Directory (CWD).
// It returns the file path and an error (if any).
func GenerateAndWriteQuickstartConfig(strategy *auth0.FileOutputStrategy, envValues map[string]string, tenantDomain string, client *management.Client, port int) (string, error) {
	resolvedEnv, err := replaceDetectionSub(envValues, tenantDomain, client, port)
	if err != nil {
		return "", err
	}

	if strategy == nil {
		strategy = &auth0.FileOutputStrategy{Path: ".env", Format: "dotenv"}
	}

	dir := filepath.Dir(strategy.Path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory structure %s: %w", dir, err)
		}
	}

	var contentBuilder strings.Builder

	switch strategy.Format {
	case "dotenv":
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("%s=\"%s\"\n", key, resolvedEnv[key]))
		}

	case "properties":
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("%s=%s\n", key, resolvedEnv[key]))
		}

	case "yaml":
		// Produce nested YAML from dot-delimited keys (e.g. Spring Boot application.yml).
		nested := buildNestedMap(resolvedEnv)
		yamlBytes, err := yaml.Marshal(nested)
		if err != nil {
			return "", fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
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
			return "", fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(yamlBytes)

	case "ts":
		contentBuilder.WriteString("export const environment = {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("  %s: '%s',\n", key, strings.ReplaceAll(resolvedEnv[key], "'", "\\'")))
		}
		contentBuilder.WriteString("};\n")

	case "angular-ts":
		// Angular SPA environment.ts nests domain and clientId under an auth0 object,
		// matching the official Angular quickstart: environment.auth0.domain / .clientId.
		contentBuilder.WriteString("export const environment = {\n")
		contentBuilder.WriteString("  production: false,\n")
		contentBuilder.WriteString("  auth0: {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("    %s: '%s',\n", key, strings.ReplaceAll(resolvedEnv[key], "'", "\\'")))
		}
		contentBuilder.WriteString("  },\n")
		contentBuilder.WriteString("};\n")

	case "dart":
		contentBuilder.WriteString("const Map<String, String> authConfig = {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("  '%s': '%s',\n", strings.ReplaceAll(key, "'", "\\'"), strings.ReplaceAll(resolvedEnv[key], "'", "\\'")))
		}
		contentBuilder.WriteString("};\n")

	case "json":
		// C# appsettings.json expects nested JSON: {"Auth0": {"Domain": "...", "ClientId": "..."}}.
		auth0Section := make(map[string]string)
		for key, val := range resolvedEnv {
			if !strings.HasPrefix(key, "Auth0:") {
				return "", fmt.Errorf("json formatter: key %q is missing required \"Auth0:\" prefix", key)
			}
			cleanKey := strings.TrimPrefix(key, "Auth0:")
			auth0Section[cleanKey] = val
		}
		jsonBody := map[string]interface{}{"Auth0": auth0Section}
		jsonBytes, err := json.MarshalIndent(jsonBody, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(jsonBytes)

	case "xml":
		// ASP.NET OWIN Web.config.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		contentBuilder.WriteString("<configuration>\n")
		contentBuilder.WriteString("  <appSettings>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("    <add key=\"%s\" value=\"%s\" />\n", xmlEscape(key), xmlEscape(resolvedEnv[key])))
		}
		contentBuilder.WriteString("  </appSettings>\n")
		contentBuilder.WriteString("</configuration>\n")

	case "webxml":
		// Java servlet web.xml context-param entries (mvc-auth-commons).
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString("<context-param>\n")
			contentBuilder.WriteString(fmt.Sprintf("  <param-name>%s</param-name>\n", xmlEscape(key)))
			contentBuilder.WriteString(fmt.Sprintf("  <param-value>%s</param-value>\n", xmlEscape(resolvedEnv[key])))
			contentBuilder.WriteString("</context-param>\n")
		}

	case "javaee-webxml":
		// Java EE web.xml JNDI env-entry elements (auth0-java-mvc-commons).
		// Values are looked up via InitialContext.lookup("auth0.domain") etc.
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString("<env-entry>\n")
			contentBuilder.WriteString(fmt.Sprintf("  <env-entry-name>%s</env-entry-name>\n", xmlEscape(key)))
			contentBuilder.WriteString("  <env-entry-type>java.lang.String</env-entry-type>\n")
			contentBuilder.WriteString(fmt.Sprintf("  <env-entry-value>%s</env-entry-value>\n", xmlEscape(resolvedEnv[key])))
			contentBuilder.WriteString("</env-entry>\n")
		}

	case "android-strings":
		// Android res/values/strings.xml - Auth0 SDK reads credentials via string resources.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		contentBuilder.WriteString("<resources>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("    <string name=\"%s\">%s</string>\n", xmlEscape(key), xmlEscape(resolvedEnv[key])))
		}
		contentBuilder.WriteString("</resources>\n")

	case "plist":
		// IOS Auth0.plist - Auth0 Swift SDK reads ClientId and Domain from this plist.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		contentBuilder.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
		contentBuilder.WriteString("<plist version=\"1.0\">\n")
		contentBuilder.WriteString("<dict>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("    <key>%s</key>\n", key))
			contentBuilder.WriteString(fmt.Sprintf("    <string>%s</string>\n", xmlEscape(resolvedEnv[key])))
		}
		contentBuilder.WriteString("</dict>\n")
		contentBuilder.WriteString("</plist>\n")
	}

	if err := os.WriteFile(strategy.Path, []byte(contentBuilder.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write config file %s: %w", strategy.Path, err)
	}

	return strategy.Path, nil
}
