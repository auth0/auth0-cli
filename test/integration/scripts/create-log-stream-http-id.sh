#! /bin/bash

FILE=./test/integration/identifiers/log-stream-http-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create http --name integration-test-http --endpoint "https://example.com/webhook/logs" --type "application/json" --format "JSONLINES" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
