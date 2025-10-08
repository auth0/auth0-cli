#! /bin/bash

FILE=./test/integration/identifiers/resource-server-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

# First, ensure the API exists (cross-domain dependency)
api_identifier=$(./test/integration/scripts/get-api-id.sh)

# Now create the resource server app using the API's identifier
app=$( auth0 apps create -n integration-test-app-resourceserver1 -t resource_server --resource-server-identifier http://integration-test-api-newapi --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > $FILE
cat $FILE
