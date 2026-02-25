package cli

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
		Help:       "Type of quickstart: " + strings.Join(supportedQuickstartTypes, ", "),
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
		Help: fmt.Sprintf("Port number for the application (default: %d for vite, %d for nextjs, %d for jhipster)",
			(&ViteSetupStrategy{}).GetDefaultPort(), (&NextjsSetupStrategy{}).GetDefaultPort(), (&JHipsterSetupStrategy{}).GetDefaultPort()),
	}
)

type QuickstartSetupInputs struct {
	Name string
	Port int
}

// QuickstartSetupStrategy defines the interface for type-specific setup workflows.
// Each quickstart type implements this interface to define its complete setup procedure.
type QuickstartSetupStrategy interface {
	// GetDefaultPort returns the default port for this quickstart type.
	GetDefaultPort() int

	// GetDefaultAppName returns the default application name for this quickstart type.
	GetDefaultAppName() string

	// GetEnvFileName returns the environment file name for this quickstart type.
	GetEnvFileName() string

	// SetupResources creates all necessary Auth0 resources (clients, APIs, resource servers, etc.).
	// This method encapsulates the complete resource creation workflow for the quickstart type.
	// Complex types can create multiple resources here.
	SetupResources(ctx context.Context, cli *cli, inputs QuickstartSetupInputs) error

	// GenerateEnvFile generates the environment file content.
	// Returns the file content that should be written to the env file.
	GenerateEnvFile(cli *cli, inputs QuickstartSetupInputs) (string, error)

	// PrintNextSteps prints the post-setup instructions for the user.
	PrintNextSteps(cli *cli, inputs QuickstartSetupInputs)
}

func validatePort(port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be between 1024 and 65535)", port)
	}
	return nil
}

var supportedQuickstartTypes = []string{"vite", "nextjs", "jhipster"}

func quickstartStrategy(typeStr string) (QuickstartSetupStrategy, error) {
	switch strings.ToLower(typeStr) {
	case "vite":
		return &ViteSetupStrategy{}, nil
	case "nextjs":
		return &NextjsSetupStrategy{}, nil
	case "jhipster":
		return &JHipsterSetupStrategy{}, nil
	default:
		return nil, fmt.Errorf("unsupported quickstart type: %s (supported types: %s)", typeStr, strings.Join(supportedQuickstartTypes, ", "))
	}
}

// ViteSetupStrategy implements the setup workflow for Vite applications.
type ViteSetupStrategy struct {
	createdAppId string
}

func (s *ViteSetupStrategy) GetDefaultPort() int {
	return 5173
}

func (s *ViteSetupStrategy) GetDefaultAppName() string {
	return "My App"
}

func (s *ViteSetupStrategy) GetEnvFileName() string {
	return ".env"
}

func (s *ViteSetupStrategy) PrintNextSteps(cli *cli, inputs QuickstartSetupInputs) {
	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)
	cli.renderer.Infof("Next steps: \n"+
		"       1. Install dependencies: npm install \n"+
		"       2. Start your application: npm run dev\n"+
		"       3. Open your browser at %s", baseURL)
}

func (s *ViteSetupStrategy) SetupResources(ctx context.Context, cli *cli, inputs QuickstartSetupInputs) error {
	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)

	// Create Auth0 application with customized settings for Vite quickstart.
	client := &management.Client{
		Name:              &inputs.Name,
		AppType:           auth0.String(appTypeSPA),
		Callbacks:         &[]string{baseURL},
		AllowedLogoutURLs: &[]string{baseURL},
		AllowedOrigins:    &[]string{baseURL},
		WebOrigins:        &[]string{baseURL},
		OIDCConformant:    auth0.Bool(true),
		JWTConfiguration: &management.ClientJWTConfiguration{
			Algorithm: auth0.String("RS256"),
		},
		ClientMetadata: &map[string]interface{}{
			"created_by": "quickstart-docs-manual-cli",
		},
	}
	if err := cli.api.Client.Create(ctx, client); err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	cli.renderer.Infof("Application created successfully with Client ID: %s", client.GetClientID())
	s.createdAppId = client.GetClientID()
	return nil
}

func (s *ViteSetupStrategy) GenerateEnvFile(cli *cli, inputs QuickstartSetupInputs) (string, error) {
	if s.createdAppId == "" {
		return "", fmt.Errorf("no client created")
	}
	tenant, err := cli.Config.GetTenant(cli.tenant)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant: %w", err)
	}

	var envContent strings.Builder
	fmt.Fprintf(&envContent, "VITE_AUTH0_DOMAIN=%s\n", tenant.Domain)
	fmt.Fprintf(&envContent, "VITE_AUTH0_CLIENT_ID=%s\n", s.createdAppId)

	return envContent.String(), nil
}

// NextjsSetupStrategy implements the setup workflow for Next.js applications.
type NextjsSetupStrategy struct {
	createdAppId        string
	createdClientSecret string
}

func (s *NextjsSetupStrategy) GetDefaultPort() int {
	return 3000
}

func (s *NextjsSetupStrategy) GetDefaultAppName() string {
	return "My App"
}

func (s *NextjsSetupStrategy) GetEnvFileName() string {
	return ".env.local"
}

func (s *NextjsSetupStrategy) PrintNextSteps(cli *cli, inputs QuickstartSetupInputs) {
	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)
	cli.renderer.Infof("Next steps: \n"+
		"       1. Install dependencies: npm install \n"+
		"       2. Start your application: npm run dev\n"+
		"       3. Open your browser at %s", baseURL)
}

func (s *NextjsSetupStrategy) SetupResources(ctx context.Context, cli *cli, inputs QuickstartSetupInputs) error {
	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)
	callbackURL := fmt.Sprintf("%s/auth/callback", baseURL)

	// Create Auth0 application with customized settings for Next.js quickstart.
	client := &management.Client{
		Name:              &inputs.Name,
		AppType:           auth0.String(appTypeRegularWeb),
		Callbacks:         &[]string{callbackURL},
		AllowedLogoutURLs: &[]string{baseURL},
		OIDCConformant:    auth0.Bool(true),
		JWTConfiguration: &management.ClientJWTConfiguration{
			Algorithm: auth0.String("RS256"),
		},
		ClientMetadata: &map[string]interface{}{
			"created_by": "quickstart-docs-manual-cli",
		},
	}
	if err := cli.api.Client.Create(ctx, client); err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	cli.renderer.Infof("Application created successfully with Client ID: %s", client.GetClientID())
	s.createdAppId = client.GetClientID()
	s.createdClientSecret = client.GetClientSecret()
	return nil
}

func (s *NextjsSetupStrategy) GenerateEnvFile(cli *cli, inputs QuickstartSetupInputs) (string, error) {
	if s.createdAppId == "" {
		return "", fmt.Errorf("no client created")
	}

	tenant, err := cli.Config.GetTenant(cli.tenant)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant: %w", err)
	}

	secret, err := generateState(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate AUTH0_SECRET: %w", err)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)

	var envContent strings.Builder
	fmt.Fprintf(&envContent, "AUTH0_DOMAIN=%s\n", tenant.Domain)
	fmt.Fprintf(&envContent, "AUTH0_CLIENT_ID=%s\n", s.createdAppId)
	fmt.Fprintf(&envContent, "AUTH0_CLIENT_SECRET=%s\n", s.createdClientSecret)
	fmt.Fprintf(&envContent, "AUTH0_SECRET=%s\n", secret)
	fmt.Fprintf(&envContent, "APP_BASE_URL=%s\n", baseURL)

	return envContent.String(), nil
}

// JHipsterSetupStrategy implements the setup workflow for JHipster applications.
// It creates a Regular Web Application, ROLE_ADMIN and ROLE_USER roles,
// and an "Add Roles" post-login Action attached to the Login flow.
type JHipsterSetupStrategy struct {
	createdAppId        string
	createdClientSecret string
}

func (s *JHipsterSetupStrategy) GetDefaultPort() int {
	return 8080
}

func (s *JHipsterSetupStrategy) GetDefaultAppName() string {
	return "JHipster"
}

func (s *JHipsterSetupStrategy) GetEnvFileName() string {
	return ".auth0.env"
}

func (s *JHipsterSetupStrategy) PrintNextSteps(cli *cli, inputs QuickstartSetupInputs) {
	baseURL := fmt.Sprintf("http://localhost:%d", inputs.Port)
	cli.renderer.Infof("Next steps: \n"+
		"       1. Source the env file: source .auth0.env\n"+
		"       2. Start your application: ./mvnw\n"+
		"       3. Open your browser at %s\n"+
		"       4. Login with email: admin@jhipster.com and password: Admin@jhipster8080", baseURL)
}

func (s *JHipsterSetupStrategy) SetupResources(ctx context.Context, cli *cli, inputs QuickstartSetupInputs) error {
	if err := s.createApplication(ctx, cli, inputs); err != nil {
		return err
	}
	if err := s.enableConnectionForClient(ctx, cli); err != nil {
		return err
	}
	roles, err := s.createRoles(ctx, cli)
	if err != nil {
		return err
	}
	if err := s.createUserAndAssignRoles(ctx, cli, roles); err != nil {
		return err
	}
	action, err := s.createAndDeployAddRolesAction(ctx, cli)
	if err != nil {
		return err
	}
	if err := s.attachActionToPostLoginFlow(ctx, cli, action.GetID(), action.GetName()); err != nil {
		return err
	}

	return nil
}

func (s *JHipsterSetupStrategy) createApplication(ctx context.Context, cli *cli, inputs QuickstartSetupInputs) error {
	baseURL := fmt.Sprintf("http://localhost:%d/", inputs.Port)
	callbackURL := fmt.Sprintf("%slogin/oauth2/code/oidc", baseURL)

	client := &management.Client{
		Name:              &inputs.Name,
		AppType:           auth0.String(appTypeRegularWeb),
		Callbacks:         &[]string{callbackURL},
		AllowedLogoutURLs: &[]string{baseURL},
		OIDCConformant:    auth0.Bool(true),
		JWTConfiguration: &management.ClientJWTConfiguration{
			Algorithm: auth0.String("RS256"),
		},
		ClientMetadata: &map[string]interface{}{
			"created_by": "quickstart-docs-manual-cli",
		},
	}
	if err := cli.api.Client.Create(ctx, client); err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}
	cli.renderer.Infof("Application created successfully with Client ID: %s", client.GetClientID())
	s.createdAppId = client.GetClientID()
	s.createdClientSecret = client.GetClientSecret()

	return nil
}

func (s *JHipsterSetupStrategy) enableConnectionForClient(ctx context.Context, cli *cli) error {
	connectionName := "Username-Password-Authentication"
	connection, err := cli.api.Connection.ReadByName(ctx, connectionName)
	if err != nil {
		mErr, ok := err.(management.Error)
		if !ok || mErr.Status() != http.StatusNotFound {
			return fmt.Errorf("failed to read connection %q: %w", connectionName, err)
		}

		// Connection doesn't exist, create it with the client enabled.
		connection = &management.Connection{
			Name:           auth0.String(connectionName),
			Strategy:       auth0.String(management.ConnectionStrategyAuth0),
			EnabledClients: &[]string{s.createdAppId},
		}
		if err := cli.api.Connection.Create(ctx, connection); err != nil {
			return fmt.Errorf("failed to create connection %q: %w", connectionName, err)
		}
		cli.renderer.Infof("Connection '%s' created and enabled for application", connectionName)
		return nil
	}

	enabledClients := connection.GetEnabledClients()
	enabledClients = append(enabledClients, s.createdAppId)

	if err := cli.api.Connection.Update(ctx, connection.GetID(), &management.Connection{
		EnabledClients: &enabledClients,
	}); err != nil {
		return fmt.Errorf("failed to enable connection for application: %w", err)
	}
	cli.renderer.Infof("Connection '%s' enabled for application", connectionName)

	return nil
}

func (s *JHipsterSetupStrategy) createRoles(ctx context.Context, cli *cli) ([]*management.Role, error) {
	var roles []*management.Role
	for _, roleName := range []string{"ROLE_ADMIN", "ROLE_USER"} {
		name := roleName
		role := &management.Role{Name: &name}
		if err := cli.api.Role.Create(ctx, role); err != nil {
			return nil, fmt.Errorf("failed to create role %s: %w", roleName, err)
		}
		cli.renderer.Infof("Role '%s' created successfully", roleName)
		roles = append(roles, role)
	}

	return roles, nil
}

func (s *JHipsterSetupStrategy) createUserAndAssignRoles(ctx context.Context, cli *cli, roles []*management.Role) error {
	email := "admin@jhipster.com"
	password := "Admin@jhipster8080"
	connection := "Username-Password-Authentication"
	user := &management.User{
		Email:      &email,
		Password:   &password,
		Connection: &connection,
	}
	if err := cli.api.User.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	cli.renderer.Infof("User '%s' created successfully", email)

	if err := cli.api.User.AssignRoles(ctx, user.GetID(), roles); err != nil {
		return fmt.Errorf("failed to assign roles to user: %w", err)
	}
	cli.renderer.Infof("Roles assigned to user '%s'", email)

	return nil
}

func (s *JHipsterSetupStrategy) createAndDeployAddRolesAction(ctx context.Context, cli *cli) (*management.Action, error) {
	triggerID := "post-login"
	triggerVersion := "v3"
	actionName := "Add Roles"
	actionCode := `exports.onExecutePostLogin = async (event, api) => {
  const namespace = 'https://www.jhipster.tech';
  if (event.authorization) {
    api.idToken.setCustomClaim('preferred_username', event.user.email);
    api.idToken.setCustomClaim(` + "`${namespace}/roles`" + `, event.authorization.roles);
    api.accessToken.setCustomClaim(` + "`${namespace}/roles`" + `, event.authorization.roles);
  }
};`

	action := &management.Action{
		Name: &actionName,
		SupportedTriggers: []management.ActionTrigger{
			{
				ID:      &triggerID,
				Version: &triggerVersion,
			},
		},
		Code:    &actionCode,
		Runtime: auth0.String("node22"),
		Deploy:  auth0.Bool(true),
	}
	if err := cli.api.Action.Create(ctx, action); err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}
	cli.renderer.Infof("Action 'Add Roles' created successfully")

	if _, err := cli.api.Action.Deploy(ctx, action.GetID()); err != nil {
		return nil, fmt.Errorf("failed to deploy action: %w", err)
	}
	cli.renderer.Infof("Action 'Add Roles' deployed successfully")

	return action, nil
}

func (s *JHipsterSetupStrategy) attachActionToPostLoginFlow(ctx context.Context, cli *cli, actionID string, actionName string) error {
	triggerID := "post-login"

	existingBindings, err := cli.api.Action.Bindings(ctx, triggerID)
	if err != nil {
		return fmt.Errorf("failed to read post-login flow bindings: %w", err)
	}

	newBinding := &management.ActionBinding{
		Ref: &management.ActionBindingReference{
			Type:  auth0.String("action_id"),
			Value: auth0.String(actionID),
		},
		DisplayName: &actionName,
	}
	updatedBindings := append(existingBindings.Bindings, newBinding)

	if err := cli.api.Action.UpdateBindings(ctx, triggerID, updatedBindings); err != nil {
		return fmt.Errorf("failed to attach action to post-login flow: %w", err)
	}
	cli.renderer.Infof("Action '%s' added to PostLogin flow", actionName)

	return nil
}

func (s *JHipsterSetupStrategy) GenerateEnvFile(cli *cli, inputs QuickstartSetupInputs) (string, error) {
	if s.createdAppId == "" {
		return "", fmt.Errorf("no client created")
	}

	tenant, err := cli.Config.GetTenant(cli.tenant)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant: %w", err)
	}

	var envContent strings.Builder
	fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI=\"https://%s/\"\n", tenant.Domain)
	fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID=\"%s\"\n", s.createdAppId)
	fmt.Fprintf(&envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET=\"%s\"\n", s.createdClientSecret)
	fmt.Fprintf(&envContent, "JHIPSTER_SECURITY_OAUTH2_AUDIENCE=\"https://%s/api/v2/\"\n", tenant.Domain)

	return envContent.String(), nil
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
			"  - jhipster: For JHipster applications (creates app, roles, user, and login action)",
		Example: `  auth0 quickstarts setup --type vite
  auth0 quickstarts setup --type nextjs
  auth0 quickstarts setup --type jhipster
  auth0 quickstarts setup --type vite --name "My App"
  auth0 quickstarts setup --type nextjs --port 8080
  auth0 quickstarts setup --type jhipster --name "JHipster" --port 8080
  auth0 qs setup --type vite -n "My App" -p 5173`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if inputs.Type != "" {
				if _, err := quickstartStrategy(inputs.Type); err != nil {
					return err
				}
			}

			if err := qsType.Select(cmd, &inputs.Type, supportedQuickstartTypes, nil); err != nil {
				return err
			}

			strategy, err := quickstartStrategy(inputs.Type)
			if err != nil {
				return err
			}

			if err := cli.setupWithAuthentication(ctx); err != nil {
				return fmt.Errorf("authentication required: %w", err)
			}

			defaultName := strategy.GetDefaultAppName()
			if err := qsAppName.Ask(cmd, &inputs.Name, &defaultName); err != nil {
				return err
			}

			defaultPort := fmt.Sprintf("%d", strategy.GetDefaultPort())
			if err := qsPort.Ask(cmd, &inputs.Port, &defaultPort); err != nil {
				return err
			}

			// Validate port using common validation logic.
			if err := validatePort(inputs.Port); err != nil {
				return err
			}

			// Prepare inputs for strategy.
			setupInputs := QuickstartSetupInputs{
				Name: inputs.Name,
				Port: inputs.Port,
			}

			// Create Auth0 resources using the strategy.
			cli.renderer.Infof("Creating Auth0 resources for '%s'...", inputs.Name)
			if err := ansi.Waiting(func() error {
				return strategy.SetupResources(ctx, cli, setupInputs)
			}); err != nil {
				return err
			}

			// Generate environment file content.
			envContent, err := strategy.GenerateEnvFile(cli, setupInputs)
			if err != nil {
				return err
			}

			// Write or display environment file.
			envFileName := strategy.GetEnvFileName()
			message := fmt.Sprintf("     Proceed to overwrite '%s' file? : ", envFileName)
			if shouldCancelOverwrite(cli, cmd, envFileName, message) {
				cli.renderer.Warnf("Aborted creating %s file. Please create it manually using the following content:\n\n"+
					"─────────────────────────────────────────────────────────────\n"+"%s"+
					"─────────────────────────────────────────────────────────────\n", envFileName, envContent)
			} else {
				if err = os.WriteFile(envFileName, []byte(envContent), 0600); err != nil {
					return fmt.Errorf("failed to write %s file: %w", envFileName, err)
				}

				cli.renderer.Infof("%s file created successfully with your Auth0 configuration\n", envFileName)
			}

			strategy.PrintNextSteps(cli, setupInputs)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	qsType.RegisterString(cmd, &inputs.Type, "")
	qsAppName.RegisterString(cmd, &inputs.Name, "")
	qsPort.RegisterInt(cmd, &inputs.Port, 0)

	return cmd
}
