---
layout: default
---
## auth0 branding update

Update the custom branding settings for Universal Login

### Synopsis

Update the custom branding settings for Universal Login.

```
auth0 branding update [flags]
```

### Examples

```
auth0 branding update
auth0 branding update --accent "#FF4F40" --background "#2A2E35" 
auth0 branding update -a "#FF4F40" -b "#2A2E35" --logo "https://example.com/logo.png"
```

### Options

```
  -a, --accent string       Accent color.
  -b, --background string   Page background color
  -f, --favicon string      URL for the favicon. Must use HTTPS.
  -c, --font string         URL for the custom font. The URL must point to a font file and not a stylesheet. Must use HTTPS.
  -h, --help                help for update
  -l, --logo string         URL for the logo. Must use HTTPS.
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

* [auth0 branding](auth0_branding.md)	 - Manage branding options

