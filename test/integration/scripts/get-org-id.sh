#! /bin/bash

FILE=./test/integration/identifiers/org-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

org=$( auth0 orgs create -n integration-test-org-better -d "Integration Test Better Organization" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$org" | jq -r '.["id"]' > $FILE
cat $FILE
