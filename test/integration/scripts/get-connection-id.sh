#! /bin/bash

FILE=./test/integration/identifiers/connection-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

connection=$( auth0 connections create --name "integration-test-connection" --strategy "auth0" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$connection" | jq -r '.["id"]' > $FILE
cat $FILE
