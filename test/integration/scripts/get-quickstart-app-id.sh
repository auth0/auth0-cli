#! /bin/bash

FILE=./test/integration/identifiers/qs-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

app=$( auth0 apps create -n integration-test-app-qs -t native --description "Quickstart app" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > $FILE
cat $FILE
