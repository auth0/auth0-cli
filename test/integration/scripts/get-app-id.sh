#! /bin/bash

FILE=./test/integration/identifiers/app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

app=$( auth0 apps create -n integration-test-app-newapp -t native --description NewApp --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > $FILE
cat $FILE