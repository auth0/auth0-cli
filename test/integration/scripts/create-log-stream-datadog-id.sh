#! /bin/bash

logStream=$( auth0 logs streams create datadog --name integration-test-datadog --region eu --api-key 123233123455 --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > ./test/integration/identifiers/log-stream-datadog-id
