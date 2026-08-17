package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

// commandFlag is a serializable description of a single command flag.
type commandFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage,omitempty"`
	Type      string `json:"type,omitempty"`
	Default   string `json:"default,omitempty"`
}

// commandNode is a serializable representation of a command in the tree,
// carrying enough detail (usage, flags, auth) for an agent to invoke it.
type commandNode struct {
	Path         string        `json:"path"`
	Name         string        `json:"name"`
	Short        string        `json:"short"`
	Description  string        `json:"description,omitempty"`
	Usage        string        `json:"usage,omitempty"`
	Example      string        `json:"example,omitempty"`
	Arguments    []string      `json:"arguments,omitempty"`
	ValidArgs    []string      `json:"validArgs,omitempty"`
	Aliases      []string      `json:"aliases,omitempty"`
	Runnable     bool          `json:"runnable"`
	RequiresAuth bool          `json:"requiresAuth"`
	Flags        []commandFlag `json:"flags,omitempty"`
	Note         string        `json:"note,omitempty"`
	Subcommands  []commandNode `json:"subcommands,omitempty"`
}

// rawAPIFallbackNote points an agent to `auth0 api` when the listed flags don't
// cover what it needs, instead of guessing.
const rawAPIFallbackNote = "The flags above are everything this command supports. " +
	"If a parameter or field you need is not listed, don't guess: use `auth0 api` to make a raw " +
	"Auth0 Management API request instead (for example `auth0 api get \"clients/{id}\"` or " +
	"`auth0 api patch \"clients/{id}\" --data '{...}'`). Run `auth0 api --help` and see " +
	"https://auth0.com/docs/api/management/v2 for the available endpoints and fields."

// annotateWithRawAPINote sets the fallback note on each given node.
func annotateWithRawAPINote(nodes []commandNode) []commandNode {
	for i := range nodes {
		nodes[i].Note = rawAPIFallbackNote
	}

	return nodes
}

func commandsCmd(cli *cli) *cobra.Command {
	var (
		depth    int
		detailed bool
		flat     bool
	)

	cmd := &cobra.Command{
		Use:   "commands [command]",
		Args:  cobra.ArbitraryArgs,
		Short: "Discover every CLI command in one place, for humans and AI agents",
		Long: "List every command in a compact tree, along with a short description of what it does.\n\n" +
			"This gives you (or an AI agent) a single overview of the whole CLI surface, so the right " +
			"command can be found without inspecting each `--help` page individually.\n\n" +
			"Pass a command path to expand only that branch instead of the whole tree, for example " +
			"`auth0 commands apps` or `auth0 commands apps create`. This keeps the output focused when " +
			"you only care about one area.\n\n" +
			"Use `--flat` to list every runnable command on its own line, which is the easiest form to " +
			"scan or match an intent against. Add `--detailed` to include usage lines, flags, arguments and " +
			"whether authentication is required for each command, so you (or an agent) can construct a valid " +
			"invocation without opening each `--help` page. Use `--json` for a machine-readable representation.",
		Example: `  auth0 commands
  auth0 commands --flat
  auth0 commands apps
  auth0 commands apps create --detailed
  auth0 commands apps --json --detailed`,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()

			// If a command path is provided, scope the tree to that branch.
			start := root
			if len(args) > 0 {
				target, _, err := root.Find(args)
				if err != nil || target == root {
					return fmt.Errorf("unknown command %q for %q", strings.Join(args, " "), root.Name())
				}
				start = target
			}

			// When scoped to a specific command, describe that command
			// itself (with its subtree). At the root we list its children.
			scoped := start != root

			if flat {
				nodes := flattenCommands(start, scoped, detailed)
				if cli.json {
					return renderCommandTreeJSON(nodes)
				}
				renderCommandsFlatText(nodes, detailed)
				return nil
			}

			if cli.json {
				var tree []commandNode
				if scoped {
					tree = []commandNode{buildNode(start, 1, depth, detailed)}
					// Only add the note for a specific command, not the full dump.
					if detailed {
						tree = annotateWithRawAPINote(tree)
					}
				} else {
					tree = buildCommandTree(start, depth, detailed)
				}
				return renderCommandTreeJSON(tree)
			}

			renderCommandTreeText(start, depth, detailed)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&flat, "flat", false, "List every runnable command on its own line, best for scanning or intent matching.")
	cmd.Flags().BoolVar(&detailed, "detailed", false, "Include usage, flags, arguments and auth requirements for each command.")
	cmd.Flags().IntVar(&depth, "depth", 0, "Maximum depth to display. 0 shows all levels. Ignored with --flat.")

	return cmd
}

// buildCommandTree returns the children of cmd as nodes (depth 0 means unlimited).
func buildCommandTree(cmd *cobra.Command, maxDepth int, detailed bool) []commandNode {
	return collectChildren(cmd, 1, maxDepth, detailed)
}

func collectChildren(cmd *cobra.Command, level, maxDepth int, detailed bool) []commandNode {
	var nodes []commandNode

	for _, child := range availableChildren(cmd) {
		nodes = append(nodes, buildNode(child, level, maxDepth, detailed))
	}

	return nodes
}

// buildNode builds a node for cmd, recursing into subcommands up to maxDepth.
func buildNode(cmd *cobra.Command, level, maxDepth int, detailed bool) commandNode {
	node := commandNode{
		Path:         cmd.CommandPath(),
		Name:         cmd.Name(),
		Short:        cmd.Short,
		Runnable:     cmd.Runnable(),
		RequiresAuth: commandRequiresAuthentication(cmd.CommandPath()),
	}

	if detailed {
		// Prefer the longer description when it adds detail beyond Short.
		if long := strings.TrimSpace(cmd.Long); long != "" && long != cmd.Short {
			node.Description = long
		}
		node.Usage = cmd.UseLine()
		node.Example = strings.TrimSpace(cmd.Example)
		node.Arguments = extractArguments(cmd)
		node.ValidArgs = cmd.ValidArgs
		node.Aliases = cmd.Aliases
		node.Flags = collectFlags(cmd)
	}

	if maxDepth == 0 || level < maxDepth {
		node.Subcommands = collectChildren(cmd, level+1, maxDepth, detailed)
	}

	return node
}

// flattenCommands returns every runnable command under start as a flat list.
// When scoped is true and start is itself runnable, it is included too.
func flattenCommands(start *cobra.Command, scoped, detailed bool) []commandNode {
	var nodes []commandNode

	if scoped && start.Runnable() {
		node := buildNode(start, 1, 1, detailed)
		node.Subcommands = nil
		nodes = append(nodes, node)
	}

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range availableChildren(cmd) {
			if child.Runnable() {
				node := buildNode(child, 1, 1, detailed)
				node.Subcommands = nil
				nodes = append(nodes, node)
			}
			walk(child)
		}
	}
	walk(start)

	return nodes
}

// renderCommandsFlatText prints one command per line as "path  short". When
// detailed is set, each command's invocation detail is printed underneath.
func renderCommandsFlatText(nodes []commandNode, detailed bool) {
	for _, node := range nodes {
		line := ansi.Bold(node.Path)
		if node.Short != "" {
			line += "  " + ansi.Faint(node.Short)
		}
		fmt.Fprintln(iostream.Output, line)

		if detailed {
			printNodeDetail("    ", node)
		}
	}
}

// collectFlags returns the command's local (non-inherited) flags.
func collectFlags(cmd *cobra.Command) []commandFlag {
	var flags []commandFlag

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		// Skip hidden and --help flags; they're noise for an agent.
		if f.Hidden || f.Name == "help" {
			return
		}

		flags = append(flags, commandFlag{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Usage:     f.Usage,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
		})
	})

	if len(flags) == 0 {
		return nil
	}

	return flags
}

// argumentPlaceholder matches positional placeholders like <app-id> or [app-id].
var argumentPlaceholder = regexp.MustCompile(`^[<\[][a-zA-Z][a-zA-Z0-9_-]*[>\]]$`)

// extractArguments returns the positional arguments a command accepts by scanning
// its Use line and examples for <name>/[name] placeholders. Tokens that follow a
// flag are skipped, since they are flag values rather than positional arguments.
func extractArguments(cmd *cobra.Command) []string {
	seen := make(map[string]bool)
	var args []string

	for _, line := range append([]string{cmd.Use}, strings.Split(cmd.Example, "\n")...) {
		tokens := strings.Fields(line)
		for i, token := range tokens {
			if !argumentPlaceholder.MatchString(token) {
				continue
			}
			// Skip placeholders that are the value of a preceding flag.
			if i > 0 && strings.HasPrefix(tokens[i-1], "-") {
				continue
			}

			// Normalize to <name> form regardless of the bracket style used.
			name := "<" + strings.Trim(token, "<>[]") + ">"
			if seen[name] {
				continue
			}
			seen[name] = true
			args = append(args, name)
		}
	}

	return args
}

// renderJSONHelpIfRequested emits a command's help as JSON and returns true when
// help is requested with --json (or in agent mode). It runs before Cobra parses
// flags, so it also works for the root and namespace commands that have no --json
// flag of their own. A specific command is described in detail; the root is a
// compact overview.
func renderJSONHelpIfRequested(cli *cli, root *cobra.Command, args []string) bool {
	if !hasHelpRequest(args) || (!hasJSONRequest(args) && !cli.agentMode) {
		return false
	}

	// Drop the help/json tokens and let Cobra find the target from the rest.
	var findArgs []string
	for _, arg := range args {
		switch arg {
		case "help", "--help", "-h", "--json":
			continue
		}
		findArgs = append(findArgs, arg)
	}

	target, _, err := root.Find(findArgs)
	if err != nil || target == nil {
		target = root
	}

	detailed := target != root

	nodes := []commandNode{buildNode(target, 1, 0, detailed)}
	if detailed {
		nodes = annotateWithRawAPINote(nodes)
	}

	_ = renderCommandTreeJSON(nodes)
	return true
}

// hasHelpRequest reports whether args request help via --help/-h or the `help`
// subcommand. A bare "help" only counts in first position, not as a flag value.
func hasHelpRequest(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}

	return len(args) > 0 && args[0] == "help"
}

// hasJSONRequest reports whether the args contain the `--json` flag.
func hasJSONRequest(args []string) bool {
	return slices.Contains(args, "--json")
}

// agentModeEnvVar enables agent mode when set to a truthy value (e.g. 1 or true).
// In agent mode, `--help` emits JSON without needing an explicit --json flag.
const agentModeEnvVar = "AUTH0_AGENT_MODE"

// agentModeEnabled reports whether agent mode is enabled via AUTH0_AGENT_MODE.
func agentModeEnabled() bool {
	enabled, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(agentModeEnvVar)))
	return enabled
}

func renderCommandTreeJSON(tree []commandNode) error {
	encoder := json.NewEncoder(iostream.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(tree)
}

// renderCommandTreeText prints the tree with box-drawing connectors,
// keeping command names aligned with their short descriptions. When detailed
// is set, each runnable command's invocation detail is printed underneath.
func renderCommandTreeText(root *cobra.Command, maxDepth int, detailed bool) {
	fmt.Fprintln(iostream.Output, ansi.Bold(root.CommandPath()))
	printChildren(root, "", 1, maxDepth, detailed)
}

func printChildren(cmd *cobra.Command, prefix string, level, maxDepth int, detailed bool) {
	children := availableChildren(cmd)

	for i, child := range children {
		isLast := i == len(children)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		line := prefix + connector + ansi.Bold(child.Name())
		if child.Short != "" {
			line += "  " + ansi.Faint(child.Short)
		}
		fmt.Fprintln(iostream.Output, line)

		if detailed && child.Runnable() {
			node := buildNode(child, 1, 1, true)
			printNodeDetail(childPrefix, node)
		}

		if maxDepth == 0 || level < maxDepth {
			printChildren(child, childPrefix, level+1, maxDepth, detailed)
		}
	}
}

// printNodeDetail prints a runnable command's invocation detail (usage,
// arguments, aliases, auth and flags), each line indented under prefix and
// dimmed so it reads as secondary to the command name above it. A blank line
// closes the block so consecutive commands stay easy to scan.
func printNodeDetail(prefix string, node commandNode) {
	writeLine := func(text string) {
		fmt.Fprintln(iostream.Output, prefix+ansi.Faint(text))
	}

	if node.Usage != "" {
		writeLine("usage: " + node.Usage)
	}
	if len(node.Arguments) > 0 {
		writeLine("args:  " + strings.Join(node.Arguments, " "))
	}
	if len(node.Aliases) > 0 {
		writeLine("alias: " + strings.Join(node.Aliases, ", "))
	}

	auth := "not required"
	if node.RequiresAuth {
		auth = "required"
	}
	writeLine("auth:  " + auth)

	if len(node.Flags) > 0 {
		writeLine("flags:")
		for _, flag := range node.Flags {
			name := "--" + flag.Name
			if flag.Shorthand != "" {
				name = "-" + flag.Shorthand + ", --" + flag.Name
			}
			// A bool flag takes no value, so its type adds only noise.
			if flag.Type != "" && flag.Type != "bool" {
				name += " " + flag.Type
			}

			detail := "  " + name
			// Flag usage can span multiple lines (see `apps create --type`);
			// collapse it to one so it doesn't break out of the tree.
			if usage := strings.Join(strings.Fields(flag.Usage), " "); usage != "" {
				detail += "  " + usage
			}

			writeLine(detail)
		}
	}

	// Blank (prefix-only) line to separate this command from the next.
	writeLine("")
}

func availableChildren(cmd *cobra.Command) []*cobra.Command {
	var children []*cobra.Command
	for _, child := range cmd.Commands() {
		if !child.IsAvailableCommand() || child.IsAdditionalHelpTopicCommand() {
			continue
		}
		children = append(children, child)
	}
	return children
}
