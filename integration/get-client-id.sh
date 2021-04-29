#! /bin/bash

output=$( auth0 apps create -n newapp -t native --description newapp --format json --no-input )
app=${output:12}

echo $app | jq -r '.["client_id"]' > ./integration/apps/client-id
