---
layout: default
parent: auth0 terraform
has_toc: false
---
# auth0 terraform generate

This command is designed to streamline the process of generating Terraform configuration files for your Auth0 resources, serving as a bridge between the two.

It automatically scans your Auth0 Tenant and compiles a set of Terraform configuration files based on the existing resources and configurations.

The generated Terraform files are written in HashiCorp Configuration Language (HCL).

## Usage
```
auth0 terraform generate [flags]
```

## Examples

```

```


## Flags

```
      --force               Skip confirmation.
  -o, --output-dir string   Output directory for the generated Terraform config files. If not provided, the files will be saved in the current working directory. (default "./")
  -r, --resources strings   Resource types to generate Terraform config for. If not provided, config files for all available resources will be generated. (default [auth0_action,auth0_attack_protection,auth0_branding,auth0_client,auth0_client_grant,auth0_connection,auth0_custom_domain,auth0_organization,auth0_pages,auth0_resource_server,auth0_role,auth0_tenant])
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 terraform generate](auth0_terraform_generate.md) - Generate terraform configuration for your Auth0 Tenant


