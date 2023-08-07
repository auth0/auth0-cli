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
  -o, --output-dir string   Output directory for the generated Terraform config files. If not provided, the files will be saved in the current working directory. (default "./")
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


