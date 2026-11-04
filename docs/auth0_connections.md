---
layout: default
has_toc: false
has_children: true
---
# auth0 connections

Connections are sources of users, linking your applications to the identity providers
that authenticate them: database, social (Google, Facebook, ...), and enterprise
(SAML, OIDC, Azure AD, LDAP, ...) connections.

## Schema Discovery & JSON Input

Use '--schema' on a command to print its request payload schema, and '--data'
to provide that payload programmatically (validated against the schema before the call).

Examples:
  auth0 connections create --schema                       # Show the create payload schema
  auth0 connections create --data @connection.json        # Create from JSON file
  auth0 connections create --data '{"name":"..."}'        # Create from inline JSON

For more details: https://auth0.com/docs/api/management/v2

## Commands

- [auth0 connections create](auth0_connections_create.md) - Create a new connection
- [auth0 connections delete](auth0_connections_delete.md) - Delete a connection
- [auth0 connections enabled-clients](auth0_connections_enabled-clients.md) - Manage the clients enabled on a connection
- [auth0 connections list](auth0_connections_list.md) - List your connections
- [auth0 connections show](auth0_connections_show.md) - Show a connection
- [auth0 connections update](auth0_connections_update.md) - Update a connection

