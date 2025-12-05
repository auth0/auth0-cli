# 🚀 Universal Login Customization Guide

Welcome to the **Universal Login Customization Guide**! 🎨  
This document provides essential information on **configuring the rendering mode** and **customizing head tags** for Universal Login.

---

## ✨ Key Points to Remember

### 🔹 1. Rendering Mode Options
- `rendering_mode` can be set to **either** `"advanced"` or `"standard"`.
- The **default value** is `"standard"`.

### 🔹 2. Default Head Tags
- `default_head_tags_disabled` is a **toggle** to enable or disable **Universal Login's default head tags**.

### 🔹 3. Context Configuration
- `context_configuration` specifies a list of **context values** that are made available.
- Refer to the [official documentation](https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens) for supported values.

### 🔹 4. Head Tags Customization
- `head_tags` defines an **array of custom head tags** (e.g., scripts, stylesheets).
- **⚠️ At least one** `<script>` tag **must be included**.

### 🔹 5. Filters Configuration
- `filters` defines the conditions under which **advanced rendering mode** with custom UI is applied. By default, the configuration applies tenant-wide.
- `match_type` and at least one of the entity arrays (`clients`, `organizations`, or `domains`) must be specified.
  - `match_type` defines the matching logic:
    - `"includes_any"`: Uses custom assets if **any match**.
    - `"excludes_any"`: Excludes custom assets if **any match**.
  - `clients`: Up to 25 client objects, defined by either `id` or `metadata` key/value.
  - `organizations`: Up to 25 organization objects, defined by either `id` or `metadata`.
  - `domains`: Up to 25 domain objects, defined by either `id` or `metadata`.

### 🔹 6. Page Template Option
- `use_page_template` determines whether to render using the **tenant’s custom page template**.
  - When set to `true`, it attempts to use the custom page template (a warning is logged if not defined).
  - When set to `false` or omitted, the default template is used.
  - The **default is `false`**.

### 🔹 7. Partial Updates
- Only **explicitly declared fields** are updated.
- **Unspecified fields remain as they are**.

---

## 📄 Sample Configuration (`settings.json`)

```json
{
  "rendering_mode": "advanced",
  "context_configuration": [
    "branding.themes.default",
    "client.logo_uri",
    "client.description",
    "client.metadata.google_tracking_id",
    "screen.texts",
    "tenant.enabled_locales",
    "untrusted_data.submitted_form_data",
    "untrusted_data.authorization_params.ext-my_param"
  ],
  "head_tags": [
    {
      "tag": "script",
      "attributes": {
        "src": "https://cdn.sass.app/auth-screens/{{client.name}}.js",
        "defer": true,
        "integrity": [
          "sha256-someHash/Abc+123",
          "sha256-someHash/cDe+456"
        ]
      }
    },
    {
      "tag": "link",
      "attributes": {
        "rel": "stylesheet",
        "href": "https://cdn.sass.app/auth-screens/{{client.name}}.css"
      }
    }
  ],
  "filters": {
    "match_type": "includes_any",
    "clients": [
      { "id": "appId" },
      { "metadata": { "key": "value" } }
    ],
    "organizations": [
      { "id": "orgId" },
      { "metadata": { "key": "value" } }
    ],
    "domains": [
      { "id": "domainId" },
      { "metadata": { "key": "value" } }
    ]
  },
  "use_page_template": false
}
```


## ✅ Best Practices

- **Use `"advanced"` mode** for full customization/granular control of the login experience and to integrate your own component design system
- **Use `"standard"` mode for creating a consistent, branded experience for users. Choosing Standard mode will open a webpage
  within your browser where you can edit and preview your branding changes.For a comprehensive list of editable parameters and their values
- **Ensure `head_tags` includes at least one `<script>` tag** for proper functionality.
- **Use `context_configuration` values carefully** to avoid exposing sensitive data.
- **Always validate your JSON** before applying changes to prevent syntax errors and unexpected behavior.

---

## 📚 Additional Resources

📖 [Auth0 Universal Login Documentation](https://auth0.com/docs/customize/login-pages)  
📖 [Advanced Customization Guide](https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens)

---
