---
layout: default
parent: auth0 acul
has_toc: false
---
# auth0 acul init

Generate a new Advanced Customizations for Universal Login (ACUL) project from a template.
This command creates a new project with your choice of framework and authentication screens (login, signup, mfa, etc.). 
The generated project includes all necessary configuration and boilerplate code to get started with ACUL customizations.

## Usage
```
auth0 acul init [flags]
```

## Examples

```
  auth0 acul init <app_name>
  auth0 acul init acul-sample-app
  auth0 acul init acul-sample-app --template react --screens login,signup
  auth0 acul init acul-sample-app -t react -s login,mfa,signup
```


## Flags

```
  -s, --screens strings   Comma-separated list of screens to include in your ACUL project.
  -t, --template string   Template framework to use for your ACUL project.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 acul config](auth0_acul_config.md) - Configure Advanced Customizations for Universal Login screens.
- [auth0 acul dev](auth0_acul_dev.md) - Start development mode for ACUL project with automatic building and asset watching.
- [auth0 acul init](auth0_acul_init.md) - Generate a new ACUL project from a template
- [auth0 acul screen](auth0_acul_screen.md) - Manage individual screens for Advanced Customizations for Universal Login.


