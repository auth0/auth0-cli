#! /bin/bash

FILE=./test/integration/identifiers/client-grant-api-identifier
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

identifier="http://integration-test-api-client-grant"
auth0 apis create --name integration-test-api-client-grant --identifier "$identifier" --scopes read:todos,write:todos --json --no-input > /dev/null || true

mkdir -p ./test/integration/identifiers
echo "$identifier" > $FILE
cat $FILE
