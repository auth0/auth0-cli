#! /bin/bash -v

set -e

auth0 login \
   --domain "${AUTH0_CLI_CLIENT_DOMAIN}" \
   --client-id "${AUTH0_CLI_CLIENT_ID}" \
   --client-secret "${AUTH0_CLI_CLIENT_SECRET}"

set +e

commander test --filter "$FILTER" --dir ./test/integration

exit_code=$?

bash ./test/integration/scripts/test-cleanup.sh

exit $exit_code
