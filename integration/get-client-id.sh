#! /bin/bash

app=$( auth0 apps create -n newapp -t native --description NewApp --format json --no-input )

echo "$app" | jq -r '.["client_id"]' > ./integration/client-id
