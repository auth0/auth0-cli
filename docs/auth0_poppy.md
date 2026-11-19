---
layout: default
has_toc: false
has_children: true
---
# auth0 poppy

(Experimental) Prepares your company for personal agents, using the Personal Agent Protocol (Poppy).

Poppy settings are spread across the tenant, an API and a web app, and the discovery files on your website must repeat Auth0's values exactly.

These commands set that up, write one config file (`poppy.json`) for your web and API SDKs, and check the whole chain before an agent arrives.

**Warning:** This command is experimental and is subject to change in future versions.

## Commands

- [auth0 poppy config](auth0_poppy_config.md) - Rebuild the Poppy config file from the tenant
- [auth0 poppy setup](auth0_poppy_setup.md) - Turn on Poppy for the tenant, an API and a web app

