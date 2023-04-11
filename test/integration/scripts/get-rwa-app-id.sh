#! /bin/bash

FILE=./test/integration/identifiers/rwa-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

rwa_app=$( auth0 apps create -n integration-test-app-rwa -t regular --description "Regular Web App Test" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$rwa_app" | jq -r '.["client_id"]' > $FILE
cat $FILE
