#! /bin/bash

FILE=./test/integration/identifiers/role-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

role=$( auth0 roles create -n integration-test-role-newRole -d integration-test-role --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$role" | jq -r '.["id"]' > $FILE
cat $FILE
