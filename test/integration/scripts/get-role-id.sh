#! /bin/bash

role=$( auth0 roles create -n integration-test-role-newRole -d integration-test-role --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$role" | jq -r '.["id"]' > ./test/integration/identifiers/role-id
