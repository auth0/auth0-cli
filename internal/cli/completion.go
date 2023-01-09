package cli

import (
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/iostream"
)

func completionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Setup autocomplete features for this CLI on your terminal",
		Example: `  auth0 completion bash
  auth0 completion zsh
  auth0 completion fish
  auth0 completion powershell`,
		Long: `## Loading completions

### Bash

To load completion for the current session, run:

` + "```\n$ source <(auth0 completion bash)\n```" + `

To load completions for each session, run once:

` + "```\n# On Linux:\n$ auth0 completion bash > /etc/bash_completion.d/auth0\n\n# On MacOS:\n$ auth0 completion bash > /usr/local/etc/bash_completion.d/auth0\n```" + `

### Zsh:

If shell completion is not already enabled in your environment you will need to enable it.

You can run the following once:

` + "```\n$ echo \"autoload -U compinit; compinit\" >> ~/.zshrc\n```" + `

To load completions for each session, run once:

` + "```\n$ auth0 completion zsh > \"${fpath[1]}/_auth0\"\n```" + `

You will need to start a new shell for this setup to take effect.

### Fish:

` + "```\n$ auth0 completion fish | source\n```" + `

To load completions for each session, run once:

` + "```\n$ auth0 completion fish > ~/.config/fish/completions/auth0.fish\n```" + `

### Powershell:

` + "```\nPS> auth0 completion powershell | Out-String | Invoke-Expression\n```" + `

To load completions for every new session, run:

` + "```\nPS> auth0 completion powershell > auth0.ps1\n```" + `

and source this file from your powershell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				err := cmd.Root().GenBashCompletion(iostream.Output)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "zsh":
				err := cmd.Root().GenZshCompletion(iostream.Output)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "fish":
				err := cmd.Root().GenFishCompletion(iostream.Output, true)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "powershell":
				err := cmd.Root().GenPowerShellCompletion(iostream.Output)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			}
		},
	}

	return cmd
}
