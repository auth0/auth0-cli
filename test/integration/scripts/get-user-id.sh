#! /bin/bash

FILE=./test/integration/identifiers/user-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

user=$( auth0 users create -n integration-test-user-better -c Username-Password-Authentication -e newuser@example.com -p testUser12 -u cli-test --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$user" | jq -r '.["user_id"]' > $FILE
cat $FILE