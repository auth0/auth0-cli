---
layout: default
parent: auth0 users
has_toc: false
---
# auth0 users import

Import users from schema. Issues a Create Import Users Job. 
The file size limit for a bulk import is 500KB. You will need to start multiple imports if your data exceeds this size.

## Usage
```
auth0 users import [flags]
```

## Examples

```
  auth0 users import
  auth0 users import --connection-name "Username-Password-Authentication"
  auth0 users import --connection-name "Username-Password-Authentication" --users "[]"
  auth0 users import --connection-name "Username-Password-Authentication" --users "$(cat path/to/users.json)"
  cat path/to/users.json | auth0 users import --connection-name "Username-Password-Authentication"
  auth0 users import -c "Username-Password-Authentication" --template "Basic Example"
  auth0 users import -c "Username-Password-Authentication" --users "$(cat path/to/users.json)" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" --users "$(cat path/to/users.json)" --upsert --email-results --no-input
  cat path/to/users.json | auth0 users import -c "Username-Password-Authentication" --upsert --email-results --no-input
  auth0 users import -c "Username-Password-Authentication" -u "$(cat path/to/users.json)" --upsert --email-results
  cat path/to/users.json | auth0 users import -c "Username-Password-Authentication" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert --email-results
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=false --email-results=false
  auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=false --email-results=false
```


## Flags

```
  -c, --connection-name string   Name of the database connection this user should be created in.
      --email-results            When true, sends a completion email to all tenant owners when the job is finished. The default is true, so you must explicitly set this parameter to false if you do not want emails sent. (default true)
  -t, --template string          Name of JSON example to be used. Cannot be used if the '--users' flag is passed. Options include: 'Empty', 'Basic Example', 'Custom Password Hash Example' and 'MFA Factors Example'.
      --upsert                   When set to false, pre-existing users that match on email address, user ID, or username will fail. When set to true, pre-existing users that match on any of these fields will be updated, but only with upsertable attributes.
  -u, --users string             JSON payload that contains an array of user(s) to be imported. Cannot be used if the '--template' flag is passed.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users blocks](auth0_users_blocks.md) - Manage brute-force protection user blocks
- [auth0 users create](auth0_users_create.md) - Create a new user
- [auth0 users delete](auth0_users_delete.md) - Delete a user
- [auth0 users import](auth0_users_import.md) - Import users from schema
- [auth0 users open](auth0_users_open.md) - Open the user's settings page
- [auth0 users roles](auth0_users_roles.md) - Manage a user's roles
- [auth0 users search](auth0_users_search.md) - Search for users
- [auth0 users show](auth0_users_show.md) - Show an existing user
- [auth0 users update](auth0_users_update.md) - Update a user


