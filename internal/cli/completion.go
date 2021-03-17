package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func completionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Setup autocomplete features for this CLI on your terminal",
		Long: `completion [bash|zsh|fish|powershell]

To load completions:

Bash:

$ source <(auth0 completion bash)

# To load completions for each session, execute once:
Linux:
  $ auth0 completion bash > /etc/bash_completion.d/auth0
MacOS:
  $ auth0 completion bash > /usr/local/etc/bash_completion.d/auth0

Zsh:

# If shell completion is not already enabled in your environment you will need
# to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

# To load completions for each session, execute once:
$ auth0 completion zsh > "${fpath[1]}/_auth0"

# You will need to start a new shell for this setup to take effect.

Fish:

$ auth0 completion fish | source

# To load completions for each session, execute once:
$ auth0 completion fish > ~/.config/fish/completions/auth0.fish

Powershell:

PS> auth0 completion powershell | Out-String | Invoke-Expression

# To load completions for every new session, run:
PS> auth0 completion powershell > auth0.ps1
# and source this file from your powershell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				err := cmd.Root().GenBashCompletion(os.Stdout)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "zsh":
				err := cmd.Root().GenZshCompletion(os.Stdout)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "fish":
				err := cmd.Root().GenFishCompletion(os.Stdout, true)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			case "powershell":
				err := cmd.Root().GenPowerShellCompletion(os.Stdout)
				if err != nil {
					cli.renderer.Errorf("An unexpected error occurred while setting up completion: %v", err.Error())
				}
			}
		},
	}

	return cmd
}
