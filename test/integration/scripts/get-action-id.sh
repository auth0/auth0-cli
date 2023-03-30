#! /bin/bash

FILE=./test/integration/identifiers/action-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

action=$( auth0 actions create -n "integration-test-action" -t "post-login" -c "function() {}" -d "lodash=4.0.0" -s "SECRET=value" --json )

mkdir -p ./test/integration/identifiers
echo "$action" | jq -r '.["id"]' > $FILE
cat $FILE