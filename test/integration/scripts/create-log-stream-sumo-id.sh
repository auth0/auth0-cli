#! /bin/bash
set -e

FILE=./test/integration/identifiers/log-stream-sumo-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create sumo --name integration-test-sumo --source "https://collectors.sumologic.com/receiver/v1/http/example" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
