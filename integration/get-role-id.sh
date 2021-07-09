#! /bin/bash

role=$( auth0 roles create -n integration-test-role-newRole -d integration-test-role --format json --no-input )

mkdir -p ./integration/identifiers
echo "$role" | jq -r '.["id"]' > ./integration/identifiers/role-id
