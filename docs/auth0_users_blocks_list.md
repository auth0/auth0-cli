---
layout: default
parent: auth0 users blocks
has_toc: false
---
# auth0 users blocks list

List brute-force protection blocks for a given user by user ID, username, phone number or email.

## Usage
```
auth0 users blocks list [flags]
```

## Examples

```
  auth0 users blocks list <user-id|username|email|phone-number>
  auth0 users blocks list <user-id|username|email|phone-number> --json
  auth0 users blocks list "auth0|61b5b6e90783fa19f7c57dad
  auth0 users blocks list "frederik@travel0.com
```


## Flags

```
      --json   Output in json format.
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 users blocks list](auth0_users_blocks_list.md) - List brute-force protection blocks for a given user
- [auth0 users blocks unblock](auth0_users_blocks_unblock.md) - Remove brute-force protection blocks for users


