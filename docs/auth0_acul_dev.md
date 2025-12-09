---
layout: default
parent: auth0 acul
has_toc: false
---
# auth0 acul dev

Start development mode for an ACUL project. This command:
- Runs 'npm run build' to build the project initially
- Watches the dist directory for asset changes
- Automatically patches screen assets when new builds are created
- Supports both single screen development and all screens

The project directory must contain package.json with a build script.

DEV MODE (default):
- Requires: --port flag for the local development server
- Runs your build process (e.g., npm run screen <name>) for HMR development

CONNECTED MODE (--connected):
- Requires: --screens flag to specify screens to patch in Auth0 tenant  
- Updates advance rendering settings of the chosen screens in your Auth0 tenant
- Runs initial build and expects you to host assets locally
- Optionally runs build:watch in the background for continuous asset updates
- Watches and patches assets automatically when changes are detected

⚠️  Connected mode should only be used on stage/dev tenants, not production!

## Usage
```
auth0 acul dev [flags]
```

## Examples

```
  # Dev mode
  auth0 acul dev --port 55444
  auth0 acul dev -p 55444 --dir ./my_project
  
  # Connected mode
  auth0 acul dev --connected
  auth0 acul dev --connected --debug --dir ./my_project
  auth0 acul dev --connected --screens all
  auth0 acul dev -c --dir ./my_project
  auth0 acul dev --connected --screens login-id
  auth0 acul dev -c -s login-id,signup
```


## Flags

```
  -c, --connected         Enable connected mode to update advance rendering settings of Auth0 tenant. Use only on stage/dev tenants.
  -d, --dir string        Path to the ACUL project directory (must contain package.json).
  -p, --port string       Port for the local development server.
  -s, --screens strings   Specific screens to develop and watch.
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


