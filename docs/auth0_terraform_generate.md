---
layout: default
parent: auth0 terraform
has_toc: false
---
# auth0 terraform generate

(Experimental) This command is designed to streamline the process of generating Terraform configuration files for your Auth0 resources, serving as a bridge between the two.

It automatically scans your Auth0 Tenant and compiles a set of Terraform configuration files (HCL) based on the existing resources and configurations.

Refer to the [instructional guide](https://registry.terraform.io/providers/auth0/auth0/latest/docs/guides/generate_terraform_config) for specific details on how to use this command.

**Warning:** This command is experimental and is subject to change in future versions.

## Usage
```
auth0 terraform generate [flags]
```

## Examples

```
  auth0 tf generate
  auth0 tf generate -o tmp-auth0-tf
  auth0 tf generate -o tmp-auth0-tf -r auth0_client
  auth0 tf generate --output-dir tmp-auth0-tf --resources auth0_action,auth0_tenant,auth0_client 
```


## Flags

```
      --force               Skip confirmation.
  -o, --output-dir string   Output directory for the generated Terraform config files. If not provided, the files will be saved in the current working directory. (default "./")
  -r, --resources strings   Resource types to generate Terraform config for. If not provided, config files for all available resources will be generated. (default [auth0_action,auth0_attack_protection,auth0_branding,auth0_branding_theme,auth0_phone_provider,auth0_client,auth0_client_grant,auth0_connection,auth0_custom_domain,auth0_flow,auth0_flow_vault_connection,auth0_form,auth0_email_provider,auth0_email_template,auth0_guardian,auth0_log_stream,auth0_network_acl,auth0_organization,auth0_pages,auth0_prompt,auth0_prompt_custom_text,auth0_prompt_screen_renderer,auth0_resource_server,auth0_role,auth0_self_service_profile,auth0_tenant,auth0_trigger_actions,auth0_user_attribute_profile,auth0_prompt_screen_partial])
  -v, --tf-version string   Terraform version that ought to be used while generating the terraform files for resources. If not provided, 1.5.0 is used by default (default "1.5.0")
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


