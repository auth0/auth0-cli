#! /bin/bash

FILE=./test/integration/identifiers/log-stream-datadog-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create datadog --name integration-test-datadog --region eu --api-key 123233123455 --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
