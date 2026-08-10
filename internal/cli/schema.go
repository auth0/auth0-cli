package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// outputFlags control output or behavior, not input, so they may be combined
// with --data. Every other input flag conflicts with a whole-payload --data.
var outputFlags = map[string]bool{
	"json":         true,
	"json-compact": true,
	"csv":          true,
	"force":        true,
}

var schemaFlag = Flag{
	Name:     "Schema",
	LongForm: "schema",
	Help:     "Print the request payload schema for this command and exit. Use with --json for machine-readable output.",
}

// printOperationSchema prints the request payload schema for an operation, as
// JSON when cli.json is set and text otherwise.
func printOperationSchema(cli *cli, method, path string) error {
	var manager *openapi.SchemaManager
	if err := ansi.Waiting(func() (err error) {
		manager, err = openapi.NewSchemaManager()
		return err
	}); err != nil {
		return fmt.Errorf("failed to load OpenAPI schema: %w", err)
	}

	opSchema, err := manager.GetOperationSchema(method, path)
	if err != nil {
		return fmt.Errorf("failed to get schema for %s %s: %w", method, path, err)
	}

	if cli.json {
		output, err := opSchema.FormatAsJSON()
		if err != nil {
			return err
		}
		cli.renderer.Output(output)
		return nil
	}

	cli.renderer.Output(opSchema.FormatAsText())
	return nil
}

// markDataExclusive rejects combining a whole-payload --data with any granular
// input flag. Call after all flags are registered.
func markDataExclusive(cmd *cobra.Command) {
	if cmd.Flags().Lookup("data") == nil {
		return
	}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if isInputFlag(f.Name) {
			cmd.MarkFlagsMutuallyExclusive("data", f.Name)
		}
	})
}

// isInputFlag reports whether a flag supplies request input, as opposed to
// delivery (--data, --schema) or output (--json, --csv, --force).
func isInputFlag(name string) bool {
	switch {
	case name == "data": // How input is delivered, not input itself.
		return false
	case name == schemaFlag.LongForm: // Help-class; exits before RunE.
		return false
	case outputFlags[name]: // Output/meta.
		return false
	default:
		return true
	}
}

// setInputFlagNames returns the input flags the user explicitly set — used to
// detect conflicts with a stdin payload, which MarkFlagsMutuallyExclusive can't see.
func setInputFlagNames(cmd *cobra.Command) []string {
	var names []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Changed && isInputFlag(f.Name) {
			names = append(names, f.Name)
		}
	})
	return names
}
