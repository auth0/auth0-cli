# OpenAPI Schema Integration for Auth0 CLI

This package integrates the Auth0 Management API OpenAPI schema into the CLI to provide enhanced error messages with schema information.

## Overview

When users make API calls that result in 400 Bad Request errors, the CLI can now automatically fetch and display the expected request schema, helping users understand what went wrong and how to fix it.

## Features

- **Automatic Schema Fetching**: Downloads and caches the Auth0 Management API OpenAPI schema
- **Schema Caching**: Caches the schema locally for 24 hours to minimize network requests
- **Error Enhancement**: Automatically enhances 400 errors with schema information
- **Support for All Endpoints**: Works with any Auth0 Management API endpoint

## Usage

### Basic Usage in CLI Commands

The `enhanceAPIError` function in `internal/cli/error_enhancer.go` can be used to enhance any Management API error:

```go
if err := cli.api.Action.Create(cmd.Context(), action); err != nil {
    // Enhance the error with schema information for 400 errors
    err = enhanceAPIError(err, "POST", "/actions/actions")
    return fmt.Errorf("failed to create action: %w", err)
}
```

### Direct Usage

You can also use the error enhancer directly:

```go
import "github.com/auth0/auth0-cli/internal/openapi"

// Create an error enhancer
enhancer, err := openapi.NewErrorEnhancer()
if err != nil {
    // Handle error
}

// Enhance an error
enhanced := enhancer.EnhanceError(err, "POST", "/actions/actions")
```

## Example Output

When a 400 error occurs, users will see:

```
400 Bad Request: Invalid request body

Expected Request Schema:
=======================

Operation: Create an action

Required fields:
  - name (string): The name of an action.
  - supported_triggers (array): The list of triggers that this action supports.

Optional fields:
  - code (string): The source code of the action. (default: module.exports = () => {})
  - dependencies (array): The list of third party npm modules and their versions.
  - runtime (string): The Node runtime. (default: node22)
  - secrets (array): The list of secrets included in an action.

Constraints:
  - Minimum items: 1 (for supported_triggers)
  - Additional properties not allowed
```

## Implementation Details

### Schema Fetching

The schema is fetched from:
```
https://auth0.com/docs/oas/management/v2/management-api-oas.json
```

### Caching

- **Location**: `~/.auth0/cache/openapi-schema.json`
- **TTL**: 24 hours
- **Fallback**: If network fetch fails, uses stale cache if available

### Schema Structure

The schema parser handles:
- Path operations (GET, POST, PATCH, PUT, DELETE)
- Request body schemas
- Response schemas
- Schema references (`$ref`)
- Nested objects and arrays
- Schema constraints (minItems, maxItems, pattern, etc.)

## Testing

Run the tests:

```bash
go test ./internal/openapi/...
```

Run the demo:

```bash
go build -o /tmp/openapi-demo ./cmd/openapi-demo/main.go
/tmp/openapi-demo
```

## Files

- **schema.go**: Schema fetching, caching, and parsing
- **error_handler.go**: Error enhancement logic
- **error_enhancer.go**: CLI integration helpers
- **schema_test.go**: Tests for schema operations
- **error_handler_test.go**: Tests for error enhancement
- **example_usage.go**: Usage examples
- **cmd/openapi-demo/main.go**: Standalone demo program

## Integration with Actions Commands

To integrate with the `actions` commands (create and update):

1. Import the error enhancer in `internal/cli/actions.go`:
   ```go
   import "github.com/auth0/auth0-cli/internal/openapi"
   ```

2. Wrap API errors with enhancement:
   ```go
   // For POST /actions/actions
   if err := cli.api.Action.Create(cmd.Context(), action); err != nil {
       err = enhanceAPIError(err, "POST", "/actions/actions")
       return fmt.Errorf("failed to create action: %w", err)
   }

   // For PATCH /actions/actions/{id}
   if err := cli.api.Action.Update(cmd.Context(), id, action); err != nil {
       err = enhanceAPIError(err, "PATCH", fmt.Sprintf("/actions/actions/%s", id))
       return fmt.Errorf("failed to update action: %w", err)
   }
   ```

## Performance Considerations

- Schema fetching only happens once per 24 hours (cached)
- Error enhancement has minimal overhead (~1ms for schema lookup)
- Non-400 errors are returned immediately without processing
- If schema loading fails, the original error is returned unchanged

## Future Enhancements

Possible improvements:
- Support for request validation before API call
- Schema-based autocomplete for interactive prompts
- Validation of flag values against enum constraints
- Better formatting for complex nested schemas
- Integration with all CLI commands (not just actions)

## Limitations

- Only enhances 400 Bad Request errors
- Requires internet connection for initial schema fetch
- Schema cache may become stale if API changes significantly
- Does not validate request payloads before sending

## Contributing

When adding OpenAPI integration to new commands:

1. Use the `enhanceAPIError` helper function
2. Provide the correct HTTP method and path
3. Add tests for the error enhancement
4. Update this README with examples
