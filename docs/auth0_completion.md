---
layout: default
---
## auth0 completion

Setup autocomplete features for this CLI on your terminal

### Synopsis

completion [bash|zsh|fish|powershell]

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


```
auth0 completion
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0](/auth0-cli/)	 - Supercharge your development workflow.

