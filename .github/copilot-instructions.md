# Auth0 CLI Copilot Instructions

## Project Overview
This is the official Auth0 CLI - a Go-based command-line tool for managing Auth0 tenants, resources, and integrations. The CLI provides comprehensive management capabilities for Auth0's identity platform through a unified terminal interface.

## Architecture & Structure

### Core Components
- **`cmd/auth0/main.go`**: Entry point that delegates to `internal/cli.Execute()`
- **`internal/cli/`**: Main CLI logic with one file per resource type (apps.go, users.go, etc.)
- **`internal/auth0/`**: Auth0 Management API client wrapper and authentication
- **`internal/config/`**: Configuration management and tenant authentication state
- **`internal/display/`**: Output rendering (JSON, CSV, tables) with consistent formatting
- **`internal/prompt/`**: Interactive prompts and user input handling

### Command Structure Pattern
Each resource follows a consistent pattern in `internal/cli/`:
```go
// Define flags and arguments as package variables
var resourceName = Flag{Name: "Name", LongForm: "name", Help: "...", IsRequired: true}

// Command constructors follow naming: new{Resource}{Action}Cmd()
func newAppsListCmd(cli *cli) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "list",
        RunE:  runAppsListCmd(cli),
    }
    // Add flags using: resourceName.RegisterStringU(cmd, &inputs.Name, "")
    return cmd
}
```

### Authentication & Configuration
- **Two auth modes**: User authentication (device flow) vs Machine-to-Machine (client credentials)
- **Multi-tenant support**: Config stores multiple tenant credentials in `~/.config/auth0/config.json`
- **Token management**: Automatic refresh of expired tokens, scope validation
- **Private cloud support**: Client credentials only for private cloud tenants

## Development Workflow

### Essential Commands
```bash
# Setup
make deps                    # Download dependencies
make test-mocks             # Generate mocks with mockgen

# Development
make build                  # Build native binary to ./out/auth0
make install               # Install to $GOPATH/bin
go run ./cmd/auth0 <cmd>   # Test commands during development

# Testing
make test-unit             # Unit tests with race detection
make test-integration      # Integration tests (requires AUTH0_* env vars)
make lint                  # golangci-lint checks with auto-fix

# Documentation
make docs                  # Generate command docs from cobra commands
```

### Environment Setup
Create `.env` file for integration tests:
```bash
export AUTH0_DOMAIN="your-tenant.auth0.com"
export AUTH0_CLIENT_ID="your-m2m-client-id" 
export AUTH0_CLIENT_SECRET="your-m2m-client-secret"
```

## Code Patterns & Conventions

### Flag & Argument Definitions
```go
// Always define as package-level variables for reuse
var appName = Flag{
    Name:       "Name",
    LongForm:   "name", 
    ShortForm:  "n",
    Help:       "Application name.",
    IsRequired: true,
}

// Register in command constructor
appName.RegisterString(cmd, &inputs.Name, "")
```

### Display & Output
- **Consistent rendering**: Use `cli.renderer.Result()` for final output
- **Multiple formats**: Support JSON (`--json`), CSV (`--csv`), and table (default)
- **Interactive prompts**: Use `cli.renderer` for messages, prompts for missing required fields
- **View interface**: Implement `AsTableHeader()`, `AsTableRow()`, `Object()` for consistent display

### Error Handling
```go
// Use Auth0 SDK errors for API calls
if err != nil {
    return fmt.Errorf("failed to retrieve application: %w", err)
}

// Define domain-specific errors
var errNoApps = errors.New("there are currently no applications")
```

### Testing Patterns
- **Unit tests**: Test command logic with mocked API calls
- **Integration tests**: YAML-based test cases in `test/integration/` using `commander` CLI testing framework
- **Table testing**: Use `expectTable()` helper for output validation
- **Mocks**: Generate with `//go:generate` directives using mockgen

### Command Registration
In `internal/cli/root.go`, commands are registered in `addSubCommands()`:
```go
rootCmd.AddCommand(newAppsCmd(cli))
rootCmd.AddCommand(newUsersCmd(cli))
```

## Key Integrations

### Auth0 Management API
- Uses `github.com/auth0/go-auth0` SDK with automatic pagination
- API client configured in `cli.setupWithAuthentication()`
- Rate limiting handled by SDK, respects tenant subscription limits

### External Dependencies
- **Cobra**: CLI framework with automatic help generation
- **Survey/Promptui**: Interactive prompts for missing inputs  
- **Tablewriter**: Consistent table formatting across commands
- **Terraform**: Generate terraform configurations via `terraform generate`

## Adding New Commands

1. **Create command file**: `internal/cli/{resource}.go`
2. **Define flags**: Package-level variables following existing patterns
3. **Implement commands**: Constructor functions (`new{Resource}{Action}Cmd`)
4. **Add to root**: Register in `addSubCommands()` 
5. **Add tests**: Unit tests in `{resource}_test.go`, integration test cases in `test/integration/`
6. **Update docs**: Run `make docs` to regenerate command documentation

## Testing & Validation

- **Pre-commit**: Run `make lint test-unit` before commits
- **Integration tests**: Require live Auth0 tenant with M2M app credentials
- **Documentation**: Always regenerated from cobra command definitions
- **Coverage**: Both unit and integration test coverage tracked

## Release Process

Tags trigger automated releases via GitHub Actions:
1. Create tag: `git tag v1.2.3`  
2. Push tag: `git push origin v1.2.3`
3. GitHub Action builds binaries for all platforms and updates Homebrew formula
