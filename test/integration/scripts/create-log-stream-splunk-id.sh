#! /bin/bash

logStream=$( auth0 logs streams create splunk --name integration-test-splunk --domain "demo.splunk.com" --token "12a34ab5-c6d7-8901-23ef-456b7c89d0c1" --port "8088" --secure --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > ./test/integration/identifiers/log-stream-splunk-id
