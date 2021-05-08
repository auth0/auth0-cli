#! /bin/bash

api=$( auth0 apis create --name integration-test-api-newapi --identifier http://integration-test-api-newapi --scopes read:todos --format json --no-input )

echo "$api" | jq -r '.["id"]' > ./integration/identifiers/api-id
