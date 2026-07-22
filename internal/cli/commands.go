package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
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

// commandNode is a serializable representation of a command in the tree.
//
// It is intentionally structured so that an AI agent can, from a single
// call, discover which command does what and learn enough to invoke it
// (usage line, flags, whether authentication is needed) without having to
// call `--help` on each command individually.
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
	Subcommands  []commandNode `json:"subcommands,omitempty"`
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
		Short: "List all commands in a tree structure",
		Long: "List every command in a compact tree, along with a short description of what it does.\n\n" +
			"This gives you (or an AI agent) a single overview of the whole CLI surface, so the right " +
			"command can be found without inspecting each `--help` page individually.\n\n" +
			"Pass a command path to expand only that branch instead of the whole tree, for example " +
			"`auth0 commands apps` or `auth0 commands apps create`. This keeps the output focused when " +
			"you only care about one area.\n\n" +
			"Use `--flat` to list every runnable command on its own line, which is the easiest form to " +
			"scan or match an intent against. Use `--json` for a machine-readable representation, and add " +
			"`--detailed` to include usage lines, flags, arguments and whether authentication is required, " +
			"which is enough for an agent to construct a valid invocation on its own.",
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
				renderCommandsFlatText(nodes)
				return nil
			}

			if cli.json {
				var tree []commandNode
				if scoped {
					tree = []commandNode{buildNode(start, 1, depth, detailed)}
				} else {
					tree = buildCommandTree(start, depth, detailed)
				}
				return renderCommandTreeJSON(tree)
			}

			renderCommandTreeText(start, depth)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&flat, "flat", false, "List every runnable command on its own line, best for scanning or intent matching.")
	cmd.Flags().BoolVar(&detailed, "detailed", false, "Include usage, flags, arguments and auth requirements. Best used with --json.")
	cmd.Flags().IntVar(&depth, "depth", 0, "Maximum depth to display. 0 shows all levels. Ignored with --flat.")

	return cmd
}

// buildCommandTree converts the cobra command tree into a slice of
// commandNode, honoring the requested depth (0 means unlimited).
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

// buildNode builds a commandNode for a single command, recursing into its
// subcommands until maxDepth is reached (0 means unlimited).
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

// flattenCommands returns every runnable (leaf) command under start as a flat
// list, without nesting. This is the easiest shape for scanning or matching an
// intent against, since each command is a single self-contained entry. When
// scoped is true and start itself is runnable, it is included as well.
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

// renderCommandsFlatText prints one command per line as "path — short".
func renderCommandsFlatText(nodes []commandNode) {
	for _, node := range nodes {
		line := ansi.Bold(node.Path)
		if node.Short != "" {
			line += "  " + ansi.Faint(node.Short)
		}
		fmt.Fprintln(iostream.Output, line)
	}
}

// collectFlags returns the command's local (non-inherited) flags, so an
// agent sees only the flags meaningful to that specific command.
func collectFlags(cmd *cobra.Command) []commandFlag {
	var flags []commandFlag

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		// Skip hidden flags and the ubiquitous --help flag, which are
		// noise for an agent trying to construct an invocation.
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

// argumentPlaceholder matches positional argument placeholders written as
// <app-id> or [app-id]. The CLI documents positional arguments this way in
// its Use and Example lines, so we surface them as an explicit list.
var argumentPlaceholder = regexp.MustCompile(`^[<\[][a-zA-Z][a-zA-Z0-9_-]*[>\]]$`)

// extractArguments discovers the positional arguments a command accepts by
// scanning its Use line and examples for <name> / [name] placeholders. This
// gives an agent an explicit, deduplicated list instead of forcing it to
// parse free-form example text.
//
// A placeholder only counts as positional when it is not the value of a flag
// (for example `--description <description>` is a flag value, not a positional
// argument), so we skip any token that directly follows a flag.
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

func renderCommandTreeJSON(tree []commandNode) error {
	encoder := json.NewEncoder(iostream.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(tree)
}

// renderCommandTreeText prints the tree with box-drawing connectors,
// keeping command names aligned with their short descriptions.
func renderCommandTreeText(root *cobra.Command, maxDepth int) {
	fmt.Fprintln(iostream.Output, ansi.Bold(root.CommandPath()))
	printChildren(root, "", 1, maxDepth)
}

func printChildren(cmd *cobra.Command, prefix string, level, maxDepth int) {
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

		if maxDepth == 0 || level < maxDepth {
			printChildren(child, childPrefix, level+1, maxDepth)
		}
	}
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
