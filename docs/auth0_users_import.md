---
layout: default
---
## auth0 users import

Import users from schema

### Synopsis

Import users from schema. Issues a Create Import Users Job. 
The file size limit for a bulk import is 500KB. You will need to start multiple imports if your data exceeds this size.

```
auth0 users import [flags]
```

### Examples

```
auth0 users import
auth0 users import --connection "Username-Password-Authentication"
auth0 users import -c "Username-Password-Authentication" --template "Basic Example"
auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=true
auth0 users import -c "Username-Password-Authentication" -t "Basic Example" --upsert=true --email-results=false
```

### Options

```
  -c, --connection string   Name of the database connection this user should be created in.
  -r, --email-results       When true, sends a completion email to all tenant owners when the job is finished. The default is true, so you must explicitly set this parameter to false if you do not want emails sent. (default true)
  -h, --help                help for import
  -t, --template string     Name of JSON example to be used.
  -u, --upsert              When set to false, pre-existing users that match on email address, user ID, or username will fail. When set to true, pre-existing users that match on any of these fields will be updated, but only with upsertable attributes.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --json            Output in json format.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 users](auth0_users.md)	 - Manage resources for users

