#! /bin/bash

FILE=./test/integration/identifiers/log-stream-http-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create http --name integration-test-http --endpoint "https://example.com/webhook/logs" --type "application/json" --format "JSONLINES" --authorization "AKIAXXXXXXXXXXXXXXXX" --json --no-input )

if [ -z "$logStream" ]; then
    # Log stream unable to be created
    exit 1
fi

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
