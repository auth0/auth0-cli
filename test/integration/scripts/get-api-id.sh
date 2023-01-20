#! /bin/bash

api=$( auth0 apis create --name integration-test-api-newapi --identifier http://integration-test-api-newapi --scopes read:todos --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$api" | jq -r '.["id"]' > ./test/integration/identifiers/api-id
