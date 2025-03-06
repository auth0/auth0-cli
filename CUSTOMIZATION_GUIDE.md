# 🚀 Universal Login Customization Guide

Welcome to the **Universal Login Customization Guide**! 🎨  
This document provides essential information on **configuring the rendering mode** and **customizing head tags** for Universal Login.

---

## ✨ Key Points to Remember

### 🔹 1. Rendering Mode Options
- `rendering_mode` can be set to **either** `"advanced"` or `"standard"`.
- The **default value** is `"standard"`.

### 🔹 2. Default Head Tags
- `default_head_tags_disabled` is a **toggle** to enable/disable **Universal Login's default head tags**.

### 🔹 3. Context Configuration
- `context_configuration` contains a list of **context values** made available.
- Refer to the [official documentation](https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens) for possible values.

### 🔹 4. Head Tags Customization
- `head_tags` is an **array of custom head tags** (e.g., scripts, stylesheets).
- **⚠️ At least one** `<script>` tag **must be included**.

### 🔹 5. Updating Rendering Mode
- **Switching to `"standard"` only updates `rendering_mode`**.
- **All other fields remain unchanged**.

### 🔹 6. Partial Updates
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
  ]
}
```


## ✅ Best Practices

- **Use `"advanced"` mode** for full customization/granular control of the login experience and to integrate your own component design system
- **Use `"standard"` mode for creating a consistent, branded experience for users. Choosing Standard mode will open a webpage
  within your browser where you can edit and preview your branding changes.For a comprehensive list of editable parameters and their values
- **Ensure `head_tags` includes at least one `<script>` tag** for proper functionality.
- **When switching to `"standard"`, only update `rendering_mode`**—all other fields should remain unchanged.
- **Use `context_configuration` values carefully** to avoid exposing sensitive data.
- **Always validate your JSON** before applying changes to prevent syntax errors and unexpected behavior.

---

## 📚 Additional Resources

📖 [Auth0 Universal Login Documentation](https://auth0.com/docs/customize/login-pages)  
📖 [Advanced Customization Guide](https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens)

---
