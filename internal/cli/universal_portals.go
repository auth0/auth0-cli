package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// Temporary: these commands run a Node.js tool built into the CLI, until the plugin framework can
// install it. When it can, remove universal_portals*.go, the data/universal-portals folder (and its
// section in data/README.md) and the universalPortalsCmd line in root.go, then run "make docs".
// The framework ignores a plugin named like a built-in command, so a test fails once plugins.go exists.

// universalPortalsCmd groups the Universal Portals commands.
func universalPortalsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "universal-portals",
		Aliases: []string{"up"},
		Short:   "Set up and audit Universal Portals",
		Long: `Set up and audit Auth0 Universal Portals.

These commands run a tool that is built into the CLI. It needs Node.js 20.12 or newer, and it calls the
Auth0 Management API as the tenant you are logged in to.`,
	}

	cmd.SetUsageTemplate(namespaceUsageTemplate())
	cmd.AddCommand(
		universalPortalsToolCmd(cli, "setup",
			"Set up a Universal Portal",
			`Creates a Universal Portal with its application, forms and pages, and checks that the logged-in session has the permissions it needs.

All flags are handled by the tool. Run "auth0 universal-portals setup --help" for the list.`,
			`  auth0 universal-portals setup
  auth0 universal-portals setup --template business --name "Acme Admin" --user jane@acme.com
  auth0 up setup --dry-run --template consumer --name "Acme Account" --json
  auth0 up setup --no-input --template consumer --name "Acme Account" --json`),
		universalPortalsToolCmd(cli, "audit",
			"Check a Universal Portal",
			`Checks a Universal Portal and its application against the setup defaults, and fixes the errors the tool is able to fix.

All flags are handled by the tool. Run "auth0 universal-portals audit --help" for the list.`,
			`  auth0 universal-portals audit up_123
  auth0 universal-portals audit up_123 --fix
  auth0 up audit up_123 --organization org_123 --strict --json`),
	)

	return cmd
}

// universalPortalsToolCmd is one command of the tool. Flag parsing is off, so every argument goes to the
// tool, which has its own help and flags.
func universalPortalsToolCmd(cli *cli, name, short, long, example string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              short,
		Long:               long,
		Example:            example,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUniversalPortals(cli, name, args)
		},
	}
}

func runUniversalPortals(cli *cli, command string, args []string) error {
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf(
			"node is required but was not found. Install Node.js %d.%d or newer and try again",
			universalPortalsNodeMajor, universalPortalsNodeMinor,
		)
	}

	if err := checkUniversalPortalsNodeVersion(node); err != nil {
		return err
	}

	// The tool calls this same executable to reach the Management API.
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find the path of the CLI: %w", err)
	}

	run := universalPortalsRunFor(command, args, cli.tenant, executable, node)
	run.Stdin, run.Stdout, run.Stderr = os.Stdin, os.Stdout, os.Stderr

	code, err := runUniversalPortalsScript(run)
	if err != nil {
		return err
	}

	// The tool has already shown its own message, so the CLI only passes on the exit code.
	if code != 0 {
		os.Exit(code)
	}

	return nil
}

// universalPortalsRunFor describes one run of the tool. The tenant named by --tenant in the arguments
// wins over the one the CLI is logged in to.
func universalPortalsRunFor(command string, args []string, loggedInTenant, executable, node string) universalPortalsRun {
	env := []string{"AUTH0_CLI_BIN=" + executable}

	tenant := universalPortalsTenantFlag(args)
	if tenant == "" {
		tenant = loggedInTenant
	}
	if tenant != "" {
		env = append(env, "AUTH0_CLI_TENANT_DOMAIN="+tenant)
	}

	return universalPortalsRun{
		Node:   node,
		Script: universalPortalsScript,
		Args:   append([]string{command}, args...),
		Env:    env,
	}
}
