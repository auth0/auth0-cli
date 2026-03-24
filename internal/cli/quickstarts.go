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
	var inputs struct {
		Name          string
		App           bool
		Type          string
		Framework     string
		BuildTool     string
		Port          int
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

	cmd := &cobra.Command{
		Use:   "setup-experimental",
		Args:  cobra.NoArgs,
		Short: "Set up Auth0 for your quickstart application",
		Long: "Creates an Auth0 application and generates a .env file with the necessary configuration.\n\n" +
			"The command will:\n" +
			"  1. Check if you are authenticated (and prompt for login if needed)\n" +
			"  2. Create an Auth0 application based on the specified type\n" +
			"  3. Generate a .env file with the appropriate environment variables\n\n" +
			"Supported types are dynamically loaded from the `QuickstartConfigs` map in the codebase.",
		Example: `  auth0 quickstarts setup-experimental --type spa:react:vite
  auth0 quickstarts setup-experimental --type regular:nextjs:none
  auth0 quickstarts setup-experimental --type native:react-native:none`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := cli.setupWithAuthentication(ctx); err != nil {
				return fmt.Errorf("authentication required: %w", err)
			}

			qsConfigKey, updatedInputs, err := getQuickstartConfigKey(inputs)
			if err != nil {
				inputs = updatedInputs
				return fmt.Errorf("failed to get quickstart configuration: %w", err)
			}

			// Validate the input type against QuickstartConfigs
			config, exists := auth0.QuickstartConfigs[qsConfigKey]
			if !exists {
				return fmt.Errorf("unsupported quickstart arguments: %s. Supported types: %v", qsConfigKey, getSupportedQuickstartTypes())
			}

			// Set default values based on the selected quickstart type
			// if inputs.Name == "" {
			// 	inputs.Name = "My App"
			// }
			// if inputs.Port == 0 {
			// 	inputs.Port = 3000 // Default port, can be adjusted based on the type if needed
			// }

			// baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)

			// Create the Auth0 application

			// cli.renderer.Infof("Creating Auth0 application '%s'...", inputs.Name)
			// appType := config.RequestParams.AppType
			// callbacks := config.RequestParams.Callbacks
			// logoutURLs := config.RequestParams.AllowedLogoutURLs

			// oidcConformant := true
			// algorithm := "RS256"
			// metadata := map[string]interface{}{
			// 	"created_by": "quickstart-docs-manual-cli",
			// }

			// a := &management.Client{
			// 	Name:              &inputs.Name,
			// 	AppType:           &appType,
			// 	Callbacks:         &callbacks,
			// 	AllowedLogoutURLs: &logoutURLs,
			// 	OIDCConformant:    &oidcConformant,
			// 	JWTConfiguration: &management.ClientJWTConfiguration{
			// 		Algorithm: &algorithm,
			// 	},
			// 	ClientMetadata: &metadata,
			// }

			clients, err := generateClients(inputs, config.RequestParams)
			if err != nil {
				return fmt.Errorf("failed to generate clients: %w", err)
			}

			for _, client := range clients {
				err := ansi.Waiting(func() error {
					return cli.api.Client.Create(ctx, client)
				})

				if err != nil {
					return fmt.Errorf("failed to create application: %w", err)
				} else {
					if client.GetAppType() == "resource_server" {
						printClientDetails(client, inputs.Port, "", true)
					} else {
						// cli.renderer.Infof("Application created successfully with Client ID: %s", client.GetClientID())

						// Generate the .env file
						envFileName := ".env"
						var envContent strings.Builder
						for key, value := range config.EnvValues {
							fmt.Fprintf(&envContent, "%s=%s\n", key, value)
						}

						if err := os.WriteFile(envFileName, []byte(envContent.String()), 0600); err != nil {
							return fmt.Errorf("failed to write .env file: %w", err)
						}

						// cli.renderer.Infof("%s file created successfully with your Auth0 configuration\n", envFileName)

						printClientDetails(client, inputs.Port, envFileName, false)
					}
				}

			}

			// if err := ansi.Waiting(func() error {
			// 	return cli.api.Client.Create(ctx, a)
			// }); err != nil {
			// 	return fmt.Errorf("failed to create application: %w", err)
			// }

			// cli.renderer.Infof("Next steps: \n"+
			// 	"       1. Install dependencies: npm install \n"+
			// 	"       2. Start your application: npm run dev\n"+
			// 	"       3. Open your browser at %s", baseURL)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputs.Type, "type", "", "Type of the quickstart application (e.g., spa:react:vite, regular:nextjs:none)")
	cmd.Flags().StringVar(&inputs.Name, "name", "", "Name of the Auth0 application")
	cmd.Flags().IntVar(&inputs.Port, "port", 0, "Port number for the application")

	return cmd
}

func printClientDetails(client *management.Client, port int, configFileLocation string, isApi bool) {
	if isApi {
		// Print API-related messages
		fmt.Printf("✓  An API application \"%s\" has been created and registered\n\n", *client.Name)
		fmt.Println("✓  You can manage your API from here:")
		fmt.Printf("     https://manage.auth0.com/dashboard/#/apis/%s/settings\n", client.GetClientID())
	} else {
		// Print application-related messages
		fmt.Printf("✓  An application \"%s\" has been created in the management console\n", *client.Name)
		fmt.Printf("     Client ID: %s\n\n", client.GetClientID())

		// Print management console link
		fmt.Println("✓  You can manage your application from here:")
		fmt.Printf("     https://manage.auth0.com/dashboard/#/applications/%s/settings\n\n", client.GetClientID())

		// Print callback URLs
		if client.Callbacks != nil && len(client.GetCallbacks()) > 0 {
			fmt.Println("✓  Callback URLs registered in Auth0 Dashboard:")
			for _, callback := range client.GetCallbacks() {
				fmt.Printf("     %s\n", callback)
			}
			fmt.Println()
		}

		// Print logout URLs
		if client.AllowedLogoutURLs != nil && len(client.GetAllowedLogoutURLs()) > 0 {
			fmt.Println("✓  Logout URLs registered:")
			for _, logoutURL := range client.GetAllowedLogoutURLs() {
				fmt.Printf("     %s\n", logoutURL)
			}
			fmt.Println()
		}

		// Print config file location
		fmt.Printf("✓  Config file created: %s\n\n", configFileLocation)
	}
}

// Helper function to get supported quickstart types
func getSupportedQuickstartTypes() []string {
	var types []string
	for key := range auth0.QuickstartConfigs {
		types = append(types, key)
	}
	return types
}

// For cleaner readability, you might consider extracting this anonymous struct into a named type (e.g., type SetupInputs struct {...})
func getQuickstartConfigKey(inputs struct {
	Name          string
	App           bool
	Type          string
	Framework     string
	BuildTool     string
	Port          int
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
}) (string, struct {
	Name          string
	App           bool
	Type          string
	Framework     string
	BuildTool     string
	Port          int
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
}, error) {

	// Prompt for target resource(s) when neither flag is provided.
	if !inputs.App && !inputs.API {
		var selections []string

		err := prompt.AskMultiSelect(
			"What do you want to create? (select whatever applies)",
			&selections,
			"App",
			"API",
		)
		if err != nil {
			return "", inputs, fmt.Errorf("failed to select target resource(s): %v", err)
		}

		for _, selection := range selections {
			switch strings.ToLower(selection) {
			case "app":
				inputs.App = true
			case "api":
				inputs.API = true
			}
		}

		if !inputs.App && !inputs.API {
			return "", inputs, fmt.Errorf("please select at least one option: App and/or API")
		}
	}

	// Handle application creation inputs
	if inputs.App {
		// Prompt for --type if not provided
		if inputs.Type == "" {
			types := []string{"spa", "regular", "native", "m2m"}
			// name, message, help, options, defaultValue, required
			q := prompt.SelectInput("type", "Select the application type", "", types, "m2m", true)
			if err := prompt.AskOne(q, &inputs.Type); err != nil {
				return "", inputs, fmt.Errorf("failed to select application type: %v", err)
			}
		}

		// Prompt for --framework if not provided
		if inputs.Framework == "" {
			frameworks := []string{"react", "angular", "vue", "svelte", "nextjs", "nuxt", "flutter", "express", "django", "spring-boot", "none"}
			q := prompt.SelectInput("framework", "Select the framework", "", frameworks, "none", true)
			if err := prompt.AskOne(q, &inputs.Framework); err != nil {
				return "", inputs, fmt.Errorf("failed to select framework: %v", err)
			}
		}

		// Prompt for --build-tool if not provided (optional)
		if inputs.BuildTool == "" {
			buildTools := []string{"vite", "webpack", "cra", "none"}
			q := prompt.SelectInput("build-tool", "Select the build tool (optional)", "", buildTools, "none", false)
			if err := prompt.AskOne(q, &inputs.BuildTool); err != nil {
				return "", inputs, fmt.Errorf("failed to select build tool: %v", err)
			}
		}

		// // Set default values
		// if inputs.Name == "" {
		// 	inputs.Name = "My App"
		// }
		// if inputs.Port == 0 {
		// 	inputs.Port = 3000
		// }
		// if inputs.CallbackURL == "" {
		// 	inputs.CallbackURL = fmt.Sprintf("http://localhost:%d/callback", inputs.Port)
		// }
		// if inputs.LogoutURL == "" {
		// 	inputs.LogoutURL = fmt.Sprintf("http://localhost:%d/logout", inputs.Port)
		// }
		// if inputs.WebOriginURL == "" {
		// 	inputs.WebOriginURL = fmt.Sprintf("http://localhost:%d", inputs.Port)
		// }
	}

	// Handle API creation inputs
	if inputs.API {
		// Prompt for --identifier or --audience if not provided
		if inputs.Identifier == "" && inputs.Audience == "" {
			// name, message, help, defaultValue, required
			q := prompt.TextInput("identifier", "Enter the API identifier (or audience)", "", "", true)
			if err := prompt.AskOne(q, &inputs.Identifier); err != nil {
				return "", inputs, fmt.Errorf("failed to enter API identifier: %v", err)
			}
		}

		// Use --audience as an alias for --identifier if provided
		if inputs.Identifier == "" {
			inputs.Identifier = inputs.Audience
		}

		// Prompt for --signing-alg if not provided
		if inputs.SigningAlg == "" {
			signingAlgs := []string{"RS256", "PS256", "HS256"}
			q := prompt.SelectInput("signing-alg", "Select the signing algorithm", "", signingAlgs, "RS256", true)
			if err := prompt.AskOne(q, &inputs.SigningAlg); err != nil {
				return "", inputs, fmt.Errorf("failed to select signing algorithm: %v", err)
			}
		}

		// Prompt for --scopes if not provided
		if inputs.Scopes == "" {
			q := prompt.TextInput("scopes", "Enter the scopes (comma-separated)", "", "", false)
			if err := prompt.AskOne(q, &inputs.Scopes); err != nil {
				return "", inputs, fmt.Errorf("failed to enter scopes: %v", err)
			}
		}

		// Prompt for --token-lifetime if not provided
		if inputs.TokenLifetime == "" {
			q := prompt.TextInput("token-lifetime", "Enter the token lifetime (in seconds)", "", "86400", true)
			if err := prompt.AskOne(q, &inputs.TokenLifetime); err != nil {
				return "", inputs, fmt.Errorf("failed to enter token lifetime: %v", err)
			}
		}

		if !inputs.OfflineAccess {
			inputs.OfflineAccess = false
		}
	}

	// Construct the key to query QuickstartConfigs
	// Fallback to "none" if build tool wasn't asked/selected to match the config map keys
	buildToolKey := inputs.BuildTool
	if buildToolKey == "" {
		buildToolKey = "none"
	}

	configKey := fmt.Sprintf("%s:%s:%s", inputs.Type, inputs.Framework, buildToolKey)
	return configKey, inputs, nil
}

func generateClients(input struct {
	Name          string
	App           bool
	Type          string
	Framework     string
	BuildTool     string
	Port          int
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
}, reqParams auth0.RequestParams) ([]*management.Client, error) {
	// Prompt for the Name field if missing

	if input.Name == "" {
		input.Name = "My App"
	}

	q := prompt.TextInput("name", "Application Name", input.Name, "", true)
	if err := prompt.AskOne(q, &input.Name); err != nil {
		return nil, fmt.Errorf("failed to enter application name: %v", err)
	}

	// Default values for the client
	input.SigningAlg = "RS256"
	if input.MetaData == nil {
		input.MetaData = map[string]interface{}{
			"created_by": "quickstart-docs-manual-cli",
		}
	}

	oidcConformant := true
	// Create the base client
	baseClient := &management.Client{
		Name:              &input.Name,
		AppType:           &reqParams.AppType,
		Callbacks:         &reqParams.Callbacks,
		AllowedLogoutURLs: &reqParams.AllowedLogoutURLs,
		OIDCConformant:    &oidcConformant,
		JWTConfiguration: &management.ClientJWTConfiguration{
			Algorithm: &input.SigningAlg,
		},
		ClientMetadata: &input.MetaData,
	}

	// Generate the list of clients
	var clients []*management.Client
	clients = append(clients, baseClient)

	// Add an additional client if both App and Api are true
	if input.API {
		resourceServerAppType := "resource_server"
		q := prompt.TextInput("api_identifier", "Enter API identifier(audience)", "", "", true)
		if err := prompt.AskOne(q, &input.Name); err != nil {
			return nil, fmt.Errorf("failed to enter application identifier: %v", err)
		}
		apiClient := &management.Client{
			Name:              &input.Name,
			AppType:           &resourceServerAppType,
			Callbacks:         &reqParams.Callbacks,
			AllowedLogoutURLs: &reqParams.AllowedLogoutURLs,
			OIDCConformant:    &oidcConformant,
			JWTConfiguration: &management.ClientJWTConfiguration{
				Algorithm: &input.SigningAlg,
			},
			ClientMetadata: &input.MetaData,
		}
		clients = append(clients, apiClient)
	}

	return clients, nil
}
