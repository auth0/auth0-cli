#! /bin/bash

user=$( auth0 users create -n integration-test-user-better -c Username-Password-Authentication -e newuser@example.com -p testUser12 -u cli-test --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$user" | jq -r '.["user_id"]' > ./test/integration/identifiers/user-id
