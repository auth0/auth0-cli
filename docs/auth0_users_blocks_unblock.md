---
layout: default
parent: auth0 users blocks
has_toc: false
---
# auth0 users blocks unblock

Remove brute-force protection blocks for users by user ID, username, phone number or email.

## Usage
```
auth0 users blocks unblock [flags]
```

## Examples

```
  auth0 users blocks unblock <user-id1|username1|email1|phone-number1> <user-id2|username2|email2|phone-number2>
  auth0 users blocks unblock "auth0|61b5b6e90783fa19f7c57dad
  auth0 users blocks unblock "frederik@travel0.com" "poovam@travel0.com"
		
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


