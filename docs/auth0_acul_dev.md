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
You need to run your own build process (e.g., npm run build, npm run screen <name>) 
to generate new assets that will be automatically detected and patched.

## Usage
```
auth0 acul dev [flags]
```

## Examples

```
  auth0 acul dev
  auth0 acul dev --dir ./my_acul_project
  auth0 acul dev --screen login-id --port 3000
  auth0 acul dev -d ./project -s login-id -p 8080
```


## Flags

```
  -d, --dir string       Path to the ACUL project directory (must contain package.json).
  -p, --port string      Port for the local development server (default: 8080). (default "8080")
  -s, --screen strings   Specific screen to develop and watch. If not provided, will watch all screens in the dist/assets folder.
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


