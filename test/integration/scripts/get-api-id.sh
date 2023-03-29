#! /bin/bash

FILE=./test/integration/identifiers/api-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

api=$( auth0 apis create --name integration-test-api-newapi --identifier http://integration-test-api-newapi --scopes read:todos --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$api" | jq -r '.["id"]' > $FILE
cat $FILE
