#! /bin/bash

FILE=./test/integration/identifiers/resource-server-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

app=$( auth0 apps create -n integration-test-app-resourceserver1 -t resource_server --resource-server-identifier http://integration-test-api-newapi --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > $FILE
cat $FILE
