#! /bin/bash

FILE=./test/integration/identifiers/vault-connection-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

connection=$( auth0 flows vault connections create \
    --setup-file ./test/integration/fixtures/vault-connection.json \
    --name "integration-test-connection" \
    --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$connection" | jq -r '.["id"]' > $FILE
cat $FILE
