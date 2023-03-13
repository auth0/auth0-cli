#! /bin/bash

logStream=$( auth0 logs streams create sumo --name integration-test-sumo --source "demo.sumo.com" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > ./test/integration/identifiers/log-stream-sumo-id
