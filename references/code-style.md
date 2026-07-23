# Code Style

## Enforced tooling

`golangci-lint` v2 (`.golangci.yml`) is the gate. Enabled linters: `errcheck`, `gocritic`, `godot`, `revive`, `staticcheck` (all checks), `unconvert`, `unused`, `whitespace`. Formatters: `gofmt` with `simplify`, and `goimports` with local prefix `github.com/auth0/auth0-cli` (local imports grouped last).

- `godot`: comments must be full sentences — capitalized, ending in a period.
- `errcheck`: check returned errors (the config exempts some via exclusion rules, but prefer handling them).

## Naming conventions

- Standard Go: `PascalCase` for exported identifiers, `camelCase` for unexported, short receiver names.
- Command files are named after the resource: `apps.go`, `apis.go`, `custom_domains.go`; their tests are `<name>_test.go`.
- Flags are declared as package-level `Flag` structs (see below), named `<command><Field>` (e.g. `loginClientID`).

## The command pattern

Commands are Cobra constructors that take the shared `*cli` struct and return a `*cobra.Command`. Flags are declared declaratively:

**✅ Good** — declarative flag, wired into a Cobra command:

```go
var loginClientID = Flag{
	Name:       "Client ID",
	LongForm:   "client-id",
	Help:       "Client ID of the application when authenticating via client credentials.",
	IsRequired: false,
}

func loginCmd(cli *cli) *cobra.Command {
	var inputs LoginInputs
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate to your tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd.Context(), cli, &inputs)
		},
	}
	loginClientID.RegisterString(cmd, &inputs.ClientID, "")
	return cmd
}
```

**❌ Bad** — hardcoded flag strings, ignored error, no help text:

```go
func loginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{Use: "login", Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("client-id") // errcheck: unchecked error
		doLogin(id)                                  // no context, no error return
	}}
	cmd.Flags().String("client-id", "", "") // no help; not a Flag struct
	return cmd
}
```

## Dominant patterns

- **Dependency injection via the `cli` struct** (`internal/cli/cli.go`) — carries the renderer, analytics tracker, config, and API client; passed to every command constructor.
- **`RunE` returning errors** rather than `Run` + `os.Exit`; errors bubble to the root command.
- **Rendering through `internal/display`** — never `fmt.Println` results directly; use the renderer so JSON/table/format flags work.
