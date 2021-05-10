#! /bin/bash

app=$( auth0 apps create -n integration-test-app-newapp -t native --description NewApp --format json --no-input )

mkdir -p ./integration/identifiers
echo "$app" | jq -r '.["client_id"]' > ./integration/identifiers/app-id
