---
layout: default
has_toc: false
---
# auth0 completion

## Loading completions

### Bash

To load completion for the current session, run:

```
$ source <(auth0 completion bash)
```

To load completions for each session, run once:

```
# On Linux:
$ auth0 completion bash > /etc/bash_completion.d/auth0

# On MacOS:
$ auth0 completion bash > /usr/local/etc/bash_completion.d/auth0
```

### Zsh:

If shell completion is not already enabled in your environment you will need to enable it.

You can run the following once:

```
$ echo "autoload -U compinit; compinit" >> ~/.zshrc
```

To load completions for each session, run once:

```
$ auth0 completion zsh > "${fpath[1]}/_auth0"
```

You will need to start a new shell for this setup to take effect.

### Fish:

```
$ auth0 completion fish | source
```

To load completions for each session, run once:

```
$ auth0 completion fish > ~/.config/fish/completions/auth0.fish
```

### Powershell:

```
PS> auth0 completion powershell | Out-String | Invoke-Expression
```

To load completions for every new session, run:

```
PS> auth0 completion powershell > auth0.ps1
```

and source this file from your powershell profile.


## Usage
```
auth0 completion [bash|zsh|fish|powershell]
```

## Examples

```
  auth0 completion bash
  auth0 completion zsh
  auth0 completion fish
  auth0 completion powershell
```




## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


