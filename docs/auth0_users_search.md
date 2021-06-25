---
layout: default
---
## auth0 users search

Search for users

### Synopsis

Search for users. To create one try:
auth0 users create

```
auth0 users search [flags]
```

### Examples

```
auth0 users search
auth0 users search --query id
auth0 users search -q name --sort "name:1"
auth0 users search -q name -s "name:1"
```

### Options

```
  -h, --help           help for search
  -q, --query string   Query in Lucene query syntax. See https://auth0.com/docs/users/user-search/user-search-query-syntax for more details.
  -s, --sort string    Field to sort by. Use 'field:order' where 'order' is '1' for ascending and '-1' for descending. e.g. 'created_at:1'.
```

### Options inherited from parent commands

```
      --debug           Enable debug mode.
      --force           Skip confirmation.
      --format string   Command output format. Options: json.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```

### SEE ALSO

* [auth0 users](auth0_users.md)	 - Manage resources for users

