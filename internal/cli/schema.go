package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/openapi"
)

var schemaFlag = Flag{
	Name:     "Schema",
	LongForm: "schema",
	Help:     "Print the request payload schema for this command and exit. Use with --json for machine-readable output.",
}

// printOperationSchema loads the OpenAPI schema and prints the request payload
// for the given operation. Output is JSON when cli.json is set, text otherwise.
// Commands pass their own method and path, which are the single source of truth
// for the endpoint they call.
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

// markInputJSONExclusive marks --input-json as mutually exclusive with each of
// the given granular input flags. The pairings are individual so the granular
// flags can still be combined with one another, only not with --input-json.
func markInputJSONExclusive(cmd *cobra.Command, flags ...string) {
	for _, f := range flags {
		cmd.MarkFlagsMutuallyExclusive("input-json", f)
	}
}
