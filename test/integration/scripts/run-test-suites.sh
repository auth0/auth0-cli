#! /bin/bash -v

set -e

if [[ -z "${AUTH0_CLI_CLIENT_DOMAIN}" || -z "${AUTH0_CLI_CLIENT_ID}" || -z "${AUTH0_CLI_CLIENT_SECRET}"  ]]; then
   echo "Error: AUTH0_CLI_CLIENT_DOMAIN, AUTH0_CLI_CLIENT_ID and AUTH0_CLI_CLIENT_SECRET environment variables need to be set"
   exit 1
fi

auth0 login \
   --domain "${AUTH0_CLI_CLIENT_DOMAIN}" \
   --client-id "${AUTH0_CLI_CLIENT_ID}" \
   --client-secret "${AUTH0_CLI_CLIENT_SECRET}"

set +e

commander test --filter "$FILTER" --dir ./test/integration

exit_code=$?

bash ./test/integration/scripts/test-cleanup.sh

auth0 logout $AUTH0_CLI_CLIENT_DOMAIN

exit $exit_code
