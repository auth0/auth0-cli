#! /bin/bash

logStream=$( auth0 logs streams create http --name integration-test-http --endpoint "https://example.com/webhook/logs" --type "application/json" --format "JSONLINES" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > ./test/integration/identifiers/log-stream-http-id
