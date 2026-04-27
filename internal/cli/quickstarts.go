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
	Audience      string
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

			// ── Step 1: Decide what to create (App / API / both) ─────────────.
			if !inputs.App && !inputs.API {
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

			// ── Step 2: Auto-detect project framework ─────────────────────────.
			if inputs.App {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				detection := DetectProject(cwd)

				typeFromFlag := cmd.Flags().Changed("type")
				frameworkFromFlag := cmd.Flags().Changed("framework")

				switch {
				case inputs.Type == "m2m":
					// M2M apps have no framework or port; skip detection entirely so that
					// signal files in the directory cannot override the explicit --type flag.
					if inputs.Name == "" {
						inputs.Name = detection.AppName
					}
				case typeFromFlag && frameworkFromFlag:
					// User explicitly specified type and framework via flags; skip detection UI.
					if inputs.Name == "" {
						inputs.Name = detection.AppName
					}
					// If build tool was not explicitly provided, read it from detected config
					// files (e.g. vite.config.ts) rather than defaulting to "none" statically.
					if !cmd.Flags().Changed("build-tool") && detection.BuildTool != "" {
						inputs.BuildTool = detection.BuildTool
					}
					if inputs.BundleID == "" && detection.BundleID != "" {
						inputs.BundleID = detection.BundleID
					}
				case detection.Detected:
					noInputMode := !canPrompt(cmd)
					if len(detection.AmbiguousCandidates) > 1 {
						// Multiple package.json deps matched — show partial summary and ask user to disambiguate.
						cli.renderer.InfofNoSpace("Detected in current directory")
						cli.renderer.InfofNoSpace("Framework: %s", "Could not be determined")
						cli.renderer.InfofNoSpace("App type: %s", detectionFriendlyAppType(detection.Type))
						cli.renderer.InfofNoSpace("App name: %s", detection.AppName)
						if detection.Port > 0 {
							cli.renderer.InfofNoSpace("Port: %d", detection.Port)
						}
						if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
							if inputs.Type == "" {
								inputs.Type = detection.Type
							}
							if inputs.Port == 0 {
								inputs.Port = detection.Port
							}
							if inputs.Name == "" {
								inputs.Name = detection.AppName
							}
							if inputs.BundleID == "" && detection.BundleID != "" {
								inputs.BundleID = detection.BundleID
							}
							if inputs.Framework == "" {
								if noInputMode {
									inputs.Framework = detection.AmbiguousCandidates[0]
								} else {
									q := prompt.SelectInput("framework", "Select your framework", "",
										detection.AmbiguousCandidates, detection.AmbiguousCandidates[0], true)
									if err := prompt.AskOne(q, &inputs.Framework); err != nil {
										return fmt.Errorf("failed to select framework: %w", err)
									}
								}
							}
						}
					} else if detection.Framework != "" {
						// Single clear detection — show summary and confirm.
						titleCaser := cases.Title(language.English)
						frameworkDisplay := titleCaser.String(detection.Framework)
						if detection.BuildTool != "" && detection.BuildTool != "none" {
							frameworkDisplay += " \u00b7 " + titleCaser.String(detection.BuildTool)
						}
						cli.renderer.InfofNoSpace("Detected in current directory")
						cli.renderer.InfofNoSpace("Framework: %s", frameworkDisplay)
						cli.renderer.InfofNoSpace("App type: %s", detectionFriendlyAppType(detection.Type))
						cli.renderer.InfofNoSpace("App name: %s", detection.AppName)
						if detection.Port > 0 {
							cli.renderer.InfofNoSpace("Port: %d", detection.Port)
						}

						if noInputMode || prompt.ConfirmWithDefault("Do you want to proceed with the detected values?", true) {
							if inputs.Type == "" {
								inputs.Type = detection.Type
							}
							if inputs.Framework == "" {
								inputs.Framework = detection.Framework
							}
							if inputs.BuildTool == "" || inputs.BuildTool == "none" {
								inputs.BuildTool = detection.BuildTool
							}
							if inputs.Port == 0 {
								inputs.Port = detection.Port
							}
							if inputs.Name == "" {
								inputs.Name = detection.AppName
							}
							if inputs.BundleID == "" && detection.BundleID != "" {
								inputs.BundleID = detection.BundleID
							}
						}
					}
				default:
					// No detection signal found — notify the user and pre-fill name from directory.
					if !canPrompt(cmd) && inputs.Type == "" {
						if inputs.API {
							return fmt.Errorf("auto-detection failed: when using --app and --api together with --no-input, --type must be specified")
						}
						return fmt.Errorf("auto-detection failed: provide --type to use --no-input mode")
					}
					cli.renderer.Warnf("auto-detection failed: unable to auto detect application")
					if inputs.Name == "" {
						inputs.Name = detection.AppName
					}
				}
			}

			// ── Step 3: Resolve remaining prompts for App / API ───────────────.
			// In non-interactive mode, --type alone is not enough; --framework is also required.
			if !canPrompt(cmd) && inputs.App && inputs.Type != "" && inputs.Type != "m2m" && inputs.Framework == "" {
				return fmt.Errorf("--framework is required in non-interactive mode when --type is %s: use --framework flag", inputs.Type)
			}
			qsConfigKey, updatedInputs, wasAutoSelected, err := getQuickstartConfigKey(inputs)
			if err != nil {
				return fmt.Errorf("failed to get quickstart configuration: %w", err)
			}
			inputs = updatedInputs
			if inputs.App && wasAutoSelected {
				cli.renderer.Infof("Auto-selected build tool %q for %s/%s (no exact match for 'none')", inputs.BuildTool, inputs.Type, inputs.Framework)
			}

			// ── Step 3b: Collect application name ────────────────────────────.
			if inputs.App {
				if !cmd.Flags().Changed("name") {
					defaultName := inputs.Name
					if defaultName == "" {
						defaultName = "My App"
					}
					if canPrompt(cmd) {
						q := prompt.TextInput("name", "Application name", "Name for the Auth0 application", defaultName, true)
						if err := prompt.AskOne(q, &inputs.Name); err != nil {
							return fmt.Errorf("failed to enter application name: %w", err)
						}
					} else {
						// In --no-input mode use the resolved default (directory name or "My App").
						inputs.Name = defaultName
					}
					if inputs.Name == "" {
						return fmt.Errorf("application name cannot be empty")
					}
				}
				if inputs.Name == "" {
					return fmt.Errorf("application name cannot be empty")
				}
			}

			// ── Step 3d: Prompt for port if not explicitly set ──────────────────.
			if inputs.App && inputs.Type != "native" && inputs.Type != "m2m" && !cmd.Flags().Changed("port") && canPrompt(cmd) {
				if inputs.Port == 0 {
					// Use a sensible framework-based default when detection found no port.
					switch inputs.Framework {
					case "react", "vue", "svelte", "vanilla-javascript":
						inputs.Port = 5173
					case "angular", "flutter-web":
						inputs.Port = 4200
					case "spring-boot", "vanilla-java", "java-ee":
						inputs.Port = 8080
					default:
						inputs.Port = 3000
					}
				}
				portStr := strconv.Itoa(inputs.Port)
				q := prompt.TextInput("port", "Port number", "Port the application runs on", portStr, true)
				if err := prompt.AskOne(q, &portStr); err != nil {
					return fmt.Errorf("failed to enter port: %w", err)
				}
				p, atoiErr := strconv.Atoi(portStr)
				if atoiErr != nil {
					return fmt.Errorf("invalid port %q: must be a number", portStr)
				}
				inputs.Port = p
				if inputs.Port < 1024 || inputs.Port > 65535 {
					return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", inputs.Port)
				}
				if canPrompt(cmd) && !prompt.Confirm(fmt.Sprintf("Use port %d for callback URL?", inputs.Port)) {
					return fmt.Errorf("setup cancelled: no resources were created")
				}
			}

			// Validate explicitly-passed --port value.
			if inputs.App && inputs.Type != "native" && inputs.Type != "m2m" && cmd.Flags().Changed("port") {
				if inputs.Port < 1024 || inputs.Port > 65535 {
					return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", inputs.Port)
				}
			}

			// ── Step 3c: Collect API name for API-only flow ───────────────────.
			if inputs.API && !inputs.App {
				// Collect API name if not already set (pre-fill from CWD folder name).
				if inputs.Name == "" && !cmd.Flags().Changed("name") {
					cwd, _ := os.Getwd()
					defaultName := filepath.Base(cwd)
					if defaultName == "" || defaultName == "." {
						defaultName = "my-api"
					}
					if canPrompt(cmd) {
						q := prompt.TextInput("name", "Application Name", "Name for the Auth0 API", defaultName, true)
						if err := prompt.AskOne(q, &inputs.Name); err != nil {
							return fmt.Errorf("failed to enter application name: %w", err)
						}
					} else {
						inputs.Name = defaultName
					}
				}
			}

			if inputs.API {
				// Prompt for the identifier if not explicitly provided via flag.
				if !cmd.Flags().Changed("identifier") && !cmd.Flags().Changed("audience") {
					// Compute a suggested default without pre-populating inputs.Identifier.
					defaultID := inputs.Identifier
					if defaultID == "" {
						defaultID = inputs.Audience
					}
					if defaultID == "" && inputs.Name != "" {
						slug := strings.ToLower(strings.ReplaceAll(inputs.Name, " ", "-"))
						defaultID = "https://" + slug
					}
					if canPrompt(cmd) {
						q := prompt.TextInput(
							"identifier",
							"Enter API Identifier (audience URL, identifiers must be unique within your tenant)",
							"A unique URL that identifies your API. Must be unique across your Auth0 tenant.",
							defaultID,
							true,
						)
						if err := prompt.AskOne(q, &inputs.Identifier); err != nil {
							return fmt.Errorf("failed to enter API identifier: %w", err)
						}
					} else {
						inputs.Identifier = defaultID
						if inputs.Identifier == "" {
							return fmt.Errorf("identifier is required in non-interactive mode: use --identifier or --audience flag")
						}
					}
				} else if inputs.Identifier == "" {
					inputs.Identifier = inputs.Audience
				}

				if inputs.Identifier == "" {
					return fmt.Errorf("API identifier cannot be empty: use --identifier or --audience flag")
				}

				if err := validateAPIIdentifier(inputs.Identifier); err != nil {
					return err
				}

				// If the flag was not set, prompt interactively; fall back to 86400 in non-interactive mode.
				if inputs.TokenLifetime == "" {
					if canPrompt(cmd) {
						defaultLifetime := "86400"
						q := prompt.TextInput("token-lifetime", "Access token lifetime (seconds)",
							"How long access tokens remain valid (default: 86400 = 24 hours)", defaultLifetime, true)
						if err := prompt.AskOne(q, &inputs.TokenLifetime); err != nil {
							return fmt.Errorf("failed to enter token lifetime: %w", err)
						}
						if inputs.TokenLifetime == "" {
							cli.renderer.Warnf("Token lifetime left blank; using default 86400 seconds (24 hours)")
							inputs.TokenLifetime = "86400"
						}
					} else {
						inputs.TokenLifetime = "86400"
					}
				}

				if inputs.SigningAlg == "" {
					if canPrompt(cmd) {
						signingAlgs := []string{"RS256", "PS256", "HS256"}
						q := prompt.SelectInput("signing-alg", "Select the signing algorithm", "", signingAlgs, "RS256", true)
						if err := prompt.AskOne(q, &inputs.SigningAlg); err != nil {
							return fmt.Errorf("failed to select signing algorithm: %w", err)
						}
					} else {
						inputs.SigningAlg = "RS256"
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

					if canPrompt(cmd) {
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

			// ── Step 4: Create the Auth0 application client ───────────────────.
			if inputs.App {
				config, exists := auth0.QuickstartConfigs[qsConfigKey]
				if !exists {
					return fmt.Errorf("unsupported quickstart arguments: %s. Supported types: %v", qsConfigKey, getSupportedQuickstartTypes())
				}

				// For Expo, read the production URI scheme from app.json (expo.scheme).
				// If found, register it alongside exp://localhost:19000 so that both
				// Expo Go (development) and EAS/production builds work without a manual
				// dashboard update.
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
					if expoScheme != "" {
						callbackURI := expoScheme + "://"
						logoutURI := expoScheme + ":///"
						config.RequestParams.Callbacks = append([]string{callbackURI}, config.RequestParams.Callbacks...)
						config.RequestParams.AllowedLogoutURLs = append([]string{logoutURI}, config.RequestParams.AllowedLogoutURLs...)
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

				client, err := generateClient(inputs, config.RequestParams)
				if err != nil {
					return fmt.Errorf("failed to generate client: %w", err)
				}

				if err := ansi.Waiting(func() error {
					return cli.api.Client.Create(ctx, client)
				}); err != nil {
					return fmt.Errorf("failed to create application: %w", err)
				}

				tenant, err := cli.Config.GetTenant(cli.tenant)
				if err != nil {
					return fmt.Errorf("failed to get tenant: %w", err)
				}

				envFileName, _, err := GenerateAndWriteQuickstartConfig(&config.Strategy, config.EnvValues, tenant.Domain, client, inputs.Port)
				if err != nil {
					return fmt.Errorf("failed to generate config file: %w", err)
				}
				printClientDetails(cli, client, inputs.Port, envFileName)

				// Post-setup guidance for Expo: exp://localhost:19000 only covers Expo Go.
				// Inform the user about EAS/production build requirements.
				if inputs.Framework == "expo" {
					if expoScheme != "" {
						cli.renderer.Infof("Registered %s:// (production scheme from app.json) and exp://localhost:19000 (Expo Go) as Allowed Callback URLs.", expoScheme)
						cli.renderer.Infof("For EAS production builds, ensure your app.json scheme matches %q.", expoScheme)
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
						cli.renderer.Infof("  Android: %s://%s/android/%s/callback", nativeBundleID, tenant.Domain, nativeBundleID)
						cli.renderer.Infof("  iOS:     %s://%s/ios/%s/callback", nativeBundleID, tenant.Domain, nativeBundleID)
					}
				case "maui", "dotnet-mobile":
					if nativeBundleID != "" {
						cli.renderer.Infof("Add this Allowed Callback URL in the Auth0 Dashboard:")
						cli.renderer.Infof("  %s://callback", nativeBundleID)
					}
				case "ionic-angular", "ionic-react", "ionic-vue":
					if nativeBundleID != "" {
						// Capacitor intercepts http://localhost in the WebView (already registered).
						// Surface the appId so the user can configure deep links if needed.
						cli.renderer.Infof("Capacitor app ID: %s", nativeBundleID)
						cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
					} else {
						// No Capacitor config found — remind the user where it should be.
						cli.renderer.Warnf("Could not read Capacitor app ID. Ensure capacitor.config.json or capacitor.config.ts is present in your project root.")
						cli.renderer.Infof("http://localhost is registered as the Allowed Callback URL (Capacitor WebView).")
					}
				}

				// Track the created app's client ID so we can link it to the API below.
				linkedAppClientID = client.GetClientID()
			}

			// ── Step 5: Create the Auth0 API resource server ──────────────────.
			if inputs.API {
				// API name = "<app-name>-API", fallback to identifier.
				apiName := inputs.Identifier
				if inputs.Name != "" {
					apiName = inputs.Name + "-API"
				}

				fmt.Printf("Creating API resource server %q with identifier %q...\n", apiName, inputs.Identifier)
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
					scopeList := strings.Split(inputs.Scopes, ",")
					apiScopes := make([]management.ResourceServerScope, 0, len(scopeList))
					for _, s := range scopeList {
						s = strings.TrimSpace(s)
						if s != "" {
							v := s
							apiScopes = append(apiScopes, management.ResourceServerScope{Value: &v})
						}
					}
					if len(apiScopes) > 0 {
						rs.Scopes = &apiScopes
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
			}

			return nil
		},
	}

	// App flags.
	cmd.Flags().BoolVar(&inputs.App, "app", false, "Create an Auth0 application (SPA, regular web, or native)")
	cmd.Flags().StringVar(&inputs.Name, "name", "", "Name of the Auth0 application")
	cmd.Flags().StringVar(&inputs.Type, "type", "", "Application type: spa, regular, native, or m2m")
	cmd.Flags().StringVar(&inputs.Framework, "framework", "", "Framework to configure (e.g., react, nextjs, vue, express)")
	cmd.Flags().StringVar(&inputs.BuildTool, "build-tool", "none", "Build tool used by the project (vite, webpack, cra, none)")
	cmd.Flags().IntVar(&inputs.Port, "port", 0, "Local port the application runs on (default varies by framework, e.g. 3000, 5173)")
	cmd.Flags().StringVar(&inputs.CallbackURL, "callback-url", "", "Override the allowed callback URL for the application")
	cmd.Flags().StringVar(&inputs.LogoutURL, "logout-url", "", "Override the allowed logout URL for the application")
	cmd.Flags().StringVar(&inputs.WebOriginURL, "web-origin-url", "", "Override the allowed web origin URL for the application")

	// API flags.
	cmd.Flags().BoolVar(&inputs.API, "api", false, "Create an Auth0 API resource server")
	cmd.Flags().StringVar(&inputs.Identifier, "identifier", "", "Unique URL identifier for the API (audience), e.g. https://my-api")
	cmd.Flags().StringVar(&inputs.Audience, "audience", "", "Alias for --identifier (unique audience URL for the API)")
	cmd.Flags().StringVar(&inputs.SigningAlg, "signing-alg", "", "[API] Token signing algorithm: RS256, PS256, or HS256 (leave blank to be prompted interactively)")
	cmd.Flags().StringVar(&inputs.Scopes, "scopes", "", "[API] Comma-separated list of permission scopes for the API")
	cmd.Flags().StringVar(&inputs.TokenLifetime, "token-lifetime", "", "[API] Access token lifetime in seconds (default: 86400 = 24 hours)")
	cmd.Flags().BoolVar(&inputs.OfflineAccess, "offline-access", false, "Allow offline access (enables refresh tokens)")

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

// Helper function to get supported quickstart types.
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
func getQuickstartConfigKey(inputs SetupInputs) (string, SetupInputs, bool, error) {
	// Handle application creation inputs.
	if inputs.App {
		// Validate --type if provided (Bug 12).
		validTypes := []string{"spa", "regular", "native", "m2m"}
		if inputs.Type != "" {
			valid := false
			for _, t := range validTypes {
				if inputs.Type == t {
					valid = true
					break
				}
			}
			if !valid {
				return "", inputs, false, fmt.Errorf(
					"invalid --type %q: must be one of %s",
					inputs.Type, strings.Join(validTypes, ", "),
				)
			}
		}

		// Prompt for --type if not provided.
		if inputs.Type == "" {
			q := prompt.SelectInput("type", "Select the application type", "", validTypes, "spa", true)
			if err := prompt.AskOne(q, &inputs.Type); err != nil {
				return "", inputs, false, fmt.Errorf("failed to select application type: %w", err)
			}
		}

		// M2M apps have no framework, port, or callback URLs (Bug 6).
		if inputs.Type == "m2m" {
			return "m2m:none:none", inputs, false, nil
		}

		// Prompt for --framework filtered to the selected type.
		if inputs.Framework == "" {
			frameworks := frameworksForType(inputs.Type)
			if len(frameworks) == 0 {
				return "", inputs, false, fmt.Errorf("no frameworks available for type %q", inputs.Type)
			}
			q := prompt.SelectInput("framework", "Select the framework", "", frameworks, frameworks[0], true)
			if err := prompt.AskOne(q, &inputs.Framework); err != nil {
				return "", inputs, false, fmt.Errorf("failed to select framework: %w", err)
			}
		}

		// Resolve port from framework default before prompting (Bug 11).
		// The spec says "--port: default value used if not given", so we never prompt.
		if inputs.Port == 0 {
			inputs.Port = defaultPortForFramework(inputs.Framework)
			// Port stays 0 for native apps (react-native, expo, flutter) — no port needed.
		}
	}

	// Config key is only meaningful when an app is being created.
	if !inputs.App {
		return "", inputs, false, nil
	}

	// Fallback to "none" if build tool wasn't asked/selected to match the config map keys.
	buildToolKey := inputs.BuildTool
	if buildToolKey == "" {
		buildToolKey = "none"
	}

	configKey := fmt.Sprintf("%s:%s:%s", inputs.Type, inputs.Framework, buildToolKey)

	// When build tool is "none" and no exact match exists, find the first available config
	// for this type+framework combination (e.g. spa:react only has a :vite variant).
	wasAutoSelected := false
	if _, exists := auth0.QuickstartConfigs[configKey]; !exists && buildToolKey == "none" {
		prefix := fmt.Sprintf("%s:%s:", inputs.Type, inputs.Framework)
		var candidates []string
		for k := range auth0.QuickstartConfigs {
			if strings.HasPrefix(k, prefix) {
				candidates = append(candidates, k)
			}
		}
		if len(candidates) > 0 {
			// Sort by priority (vite > webpack > cra > others alphabetically) so modern
			// build tools are preferred over legacy ones.
			buildToolPriority := map[string]int{"vite": 0, "webpack": 1, "cra": 2}
			sort.Slice(candidates, func(i, j int) bool {
				pi, pj := len(buildToolPriority)+1, len(buildToolPriority)+1
				if parts := strings.SplitN(candidates[i], ":", 3); len(parts) == 3 {
					if p, ok := buildToolPriority[parts[2]]; ok {
						pi = p
					}
				}
				if parts := strings.SplitN(candidates[j], ":", 3); len(parts) == 3 {
					if p, ok := buildToolPriority[parts[2]]; ok {
						pj = p
					}
				}
				if pi != pj {
					return pi < pj
				}
				return candidates[i] < candidates[j]
			})
			configKey = candidates[0]
			// Update inputs.BuildTool so the caller can notify the user of the auto-selection.
			parts := strings.SplitN(configKey, ":", 3)
			if len(parts) == 3 {
				inputs.BuildTool = parts[2]
			}
			wasAutoSelected = true
		}
	}

	return configKey, inputs, wasAutoSelected, nil
}

// defaultPortForFramework returns the conventional port for a given framework name.
func defaultPortForFramework(framework string) int {
	switch framework {
	case "react", "vue", "svelte", "vanilla-javascript":
		return 5173 // Vite default.
	case "angular":
		return 4200
	case "flask", "vanilla-python":
		return 5000
	case "laravel":
		return 8000
	case "spring-boot", "java-ee", "vanilla-java":
		return 8080
	default:
		return 3000
	}
}

// validateAPIIdentifier returns an error if identifier is not a valid http:// or https:// URL.
func validateAPIIdentifier(identifier string) error {
	// Err != nil from url.Parse only fires on malformed percent-encoding; the
	// host check catches bare schemes like "http://" that Parse accepts without error.
	// u.User != nil rejects URLs with embedded credentials (e.g. http://user:pass@host).
	u, err := url.Parse(identifier)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("invalid API identifier %q: must be a valid URL beginning with http:// or https://", identifier)
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

	// Override URL fields with explicit flag values when provided (Bug 7).
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
		case auth0.DetectionSubBase:
			callbacks[i] = baseURL
		}
	}
	for i, u := range logoutURLs {
		if u == auth0.DetectionSub || u == auth0.DetectionSubBase {
			logoutURLs[i] = baseURL
		}
	}
	for i, u := range webOrigins {
		if u == auth0.DetectionSub || u == auth0.DetectionSubBase {
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
		if value != auth0.DetectionSub && value != auth0.DetectionSubBase {
			updatedEnvValues[key] = value
			continue
		}

		switch key {
		case "VITE_AUTH0_DOMAIN", "AUTH0_DOMAIN", "domain", "NUXT_AUTH0_DOMAIN",
			"auth0.domain", "Auth0:Domain", "auth0:Domain", "auth0_domain",
			"EXPO_PUBLIC_AUTH0_DOMAIN", "com.auth0.domain",
			"com_auth0_domain", "Domain":
			updatedEnvValues[key] = tenantDomain

		// Express SDK specifically requires the https:// prefix.
		case "ISSUER_BASE_URL":
			updatedEnvValues[key] = "https://" + tenantDomain

		// Spring Boot okta issuer specifically requires https:// and a trailing slash.
		case "okta.oauth2.issuer":
			updatedEnvValues[key] = "https://" + tenantDomain + "/"

		case "VITE_AUTH0_CLIENT_ID", "AUTH0_CLIENT_ID", "clientId", "NUXT_AUTH0_CLIENT_ID",
			"CLIENT_ID", "auth0.clientId", "okta.oauth2.client-id", "Auth0:ClientId",
			"auth0:ClientId", "auth0_client_id", "EXPO_PUBLIC_AUTH0_CLIENT_ID", "com.auth0.clientId",
			"com_auth0_client_id", "ClientId":
			updatedEnvValues[key] = client.GetClientID()

		case "AUTH0_CLIENT_SECRET", "NUXT_AUTH0_CLIENT_SECRET", "auth0.clientSecret",
			"okta.oauth2.client-secret", "Auth0:ClientSecret", "auth0:ClientSecret",
			"auth0_client_secret", "com.auth0.clientSecret":
			updatedEnvValues[key] = client.GetClientSecret()

		case "AUTH0_SECRET", "NUXT_AUTH0_SESSION_SECRET", "SESSION_SECRET",
			"SECRET", "AUTH0_SESSION_ENCRYPTION_KEY", "AUTH0_COOKIE_SECRET":
			secret, err := generateState(32)
			if err != nil {
				return nil, fmt.Errorf("failed to generate secret for %s: %w", key, err)
			}
			updatedEnvValues[key] = secret

		case "APP_BASE_URL", "NUXT_AUTH0_APP_BASE_URL", "BASE_URL", "AUTH0_BASE_URL":
			updatedEnvValues[key] = baseURL

		case "AUTH0_REDIRECT_URI", "AUTH0_CALLBACK_URL":
			updatedEnvValues[key] = baseURL + "/callback"

		default:
			return nil, fmt.Errorf("unhandled placeholder for env key %q: add it to replaceDetectionSub", key)
		}
	}

	return updatedEnvValues, nil
}

// buildNestedMap converts a flat map with dot-delimited keys into a nested map.
// E.g. {"okta.oauth2.issuer": "x"} -> {"okta": {"oauth2": {"issuer": "x"}}}.
func buildNestedMap(flat map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range flat {
		parts := strings.Split(key, ".")
		current := result
		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = value
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

// sortedKeys returns the keys of a map in sorted order.
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
// It returns the generated file name, the file path, and an error (if any).
func GenerateAndWriteQuickstartConfig(strategy *auth0.FileOutputStrategy, envValues map[string]string, tenantDomain string, client *management.Client, port int) (string, string, error) {
	// 1. Resolve the environment variables.
	resolvedEnv, err := replaceDetectionSub(envValues, tenantDomain, client, port)
	if err != nil {
		return "", "", err
	}

	// 2. Determine output file path and format.
	if strategy == nil {
		strategy = &auth0.FileOutputStrategy{Path: ".env", Format: "dotenv"}
	}

	// 3. Ensure the directory path exists.
	dir := filepath.Dir(strategy.Path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", "", fmt.Errorf("failed to create directory structure %s: %w", dir, err)
		}
	}

	// 4. Format the file content based on the target framework's requirement.
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
			return "", "", fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
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
			return "", "", fmt.Errorf("failed to marshal YAML for %s: %w", strategy.Path, err)
		}
		contentBuilder.Write(yamlBytes)

	case "ts":
		contentBuilder.WriteString("export const environment = {\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("  %s: '%s',\n", key, strings.ReplaceAll(resolvedEnv[key], "'", "\\'")))
		}
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
			cleanKey := strings.TrimPrefix(key, "Auth0:")
			auth0Section[cleanKey] = val
		}
		jsonBody := map[string]interface{}{"Auth0": auth0Section}
		jsonBytes, err := json.MarshalIndent(jsonBody, "", "  ")
		if err != nil {
			return "", "", fmt.Errorf("failed to marshal JSON for %s: %w", strategy.Path, err)
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

	case "android-strings":
		// Android res/values/strings.xml — Auth0 SDK reads credentials via string resources.
		contentBuilder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		contentBuilder.WriteString("<resources>\n")
		for _, key := range sortedKeys(resolvedEnv) {
			contentBuilder.WriteString(fmt.Sprintf("    <string name=\"%s\">%s</string>\n", xmlEscape(key), xmlEscape(resolvedEnv[key])))
		}
		contentBuilder.WriteString("</resources>\n")

	case "plist":
		// IOS Auth0.plist — Auth0 Swift SDK reads ClientId and Domain from this plist.
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

	// 5. Write the generated content to disk.
	if err := os.WriteFile(strategy.Path, []byte(contentBuilder.String()), 0600); err != nil {
		return "", "", fmt.Errorf("failed to write config file %s: %w", strategy.Path, err)
	}

	// 6. Return the base file name and full path.
	fileName := filepath.Base(strategy.Path)
	return fileName, strategy.Path, nil
}
