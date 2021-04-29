# Continuous Integration

## Integration Tests

Integration tests can be run with:
```bash
make integration
```

`make integration` will build and run the `auth0-cli-config-generator` command which is responsible for ensuring that a valid auth0-cli config file exists before the integration tests run. If a valid auth0-cli config file doesn't exist, `auth0-cli-config-generator` will attempt to generate one based off command line flags or/and environment variables.

`make integration` will then use [commander](https://github.com/commander-cli/commander) to run tests defined in [commander.yaml](./commander.yaml)

To run integration tests as part of a CI pipeline, several environment variables need to be exported first. When these variables are set, `auth0-cli-config-generator` will generate a valid auth0-cli config file being retrieving a token for the client, removing the need to run `auth0 login`:
```bash
export AUTH0_CLI_CLIENT_NAME="integration" \
    AUTH0_CLI_CLIENT_DOMAIN="example-test-domain.au.auth0.com" \
    AUTH0_CLI_CLIENT_ID="example-client-id" \
    AUTH0_CLI_CLIENT_SECRET="example-client-secret" \
    AUTH0_CLI_REUSE_CONFIG="false" \
    AUTH0_CLI_OVERWRITE="true"
make integration
```
