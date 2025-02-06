#! /bin/bash

FILE=./test/integration/identifiers/event-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

event=$( auth0 events create -n "integration-test-event" -t webhook -s "user.created,user.deleted" -c '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}' --json)

mkdir -p ./test/integration/identifiers
echo "$event" | jq -r '.["id"]' > $FILE
cat $FILE