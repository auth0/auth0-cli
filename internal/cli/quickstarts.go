package cli

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

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
		Help:      "Name of the Auth0 application (defaults to current directory name)",
	}
	qsPort = Flag{
		Name:      "Port",
		LongForm:  "port",
		ShortForm: "p",
		Help:      "Port number for the application (default: 5173 for vite, 3000 for nextjs/fastify, 8080 for jhipster-rwa)",
	}
)

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

			var defaultName string
			var defaultPort int

			switch inputs.Type {
			case "vite":
				defaultName = "My App"
				defaultPort = 5173
			case "nextjs", "fastify":
				defaultName = "My App"
				defaultPort = 3000
			case "jhipster-rwa":
				defaultName = "JHipster"
				defaultPort = 8080
			}

			// If name is not explicitly set, ask for it or use default.
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
			var appType string
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
				appType = appTypeRegularWeb
				callbackURL := fmt.Sprintf("%s/api/auth/callback", baseURL)
				callbacks = []string{callbackURL}
				logoutURLs = []string{baseURL}
			case "fastify":
				appType = appTypeRegularWeb
				callbackURL := fmt.Sprintf("%s/auth/callback", baseURL)
				callbacks = []string{callbackURL}
				logoutURLs = []string{baseURL}
			case "jhipster-rwa":
				appType = appTypeRegularWeb
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
				fmt.Fprintf(&envContent, "AUTH0_DOMAIN=%s\n", tenant.Domain)
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_ID=%s\n", a.GetClientID())
				fmt.Fprintf(&envContent, "AUTH0_CLIENT_SECRET=%s\n", a.GetClientSecret())
			}

			envFileName := ".env"
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
				cli.renderer.Infof("Created Jhipster RWA application")
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
