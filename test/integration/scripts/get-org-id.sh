#! /bin/bash

org=$( auth0 orgs create -n integration-test-org-better -d "Integration Test Better Organization" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$org" | jq -r '.["id"]' > ./test/integration/identifiers/org-id
