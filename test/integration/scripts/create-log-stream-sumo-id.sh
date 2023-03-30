#! /bin/bash

FILE=./test/integration/identifiers/log-stream-sumo-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create sumo --name integration-test-sumo --source "demo.sumo.com" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
