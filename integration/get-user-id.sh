#! /bin/bash

user=$( auth0 users create -n integration-test-user-better -c Username-Password-Authentication -e newuser@example.com -p testUser12 --format json --no-input )

mkdir -p ./integration/identifiers
echo "$user" | jq -r '.["user_id"]' > ./integration/identifiers/user-id
