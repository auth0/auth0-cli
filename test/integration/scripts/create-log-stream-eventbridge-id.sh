#! /bin/bash

logStream=$( auth0 logs streams create eventbridge --name integration-test-eventbridge --aws-id 999999999999 --aws-region eu-west-1 --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > ./test/integration/identifiers/log-stream-eventbridge-id
