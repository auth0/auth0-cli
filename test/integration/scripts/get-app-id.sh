#! /bin/bash

app=$( auth0 apps create -n integration-test-app-newapp -t native --description NewApp --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > ./test/integration/identifiers/app-id
