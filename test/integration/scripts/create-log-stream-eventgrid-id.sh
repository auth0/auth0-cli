#! /bin/bash

FILE=./test/integration/identifiers/log-stream-eventgrid-id
if [ -f "$FILE" ]; then
    exit 0
fi

logStream=$( auth0 logs streams create eventgrid --name integration-test-eventgrid --azure-id "b69a6835-57c7-4d53-b0d5-1c6ae580b6d5" --azure-region northeurope --azure-group "azure-logs-rg" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$logStream" | jq -r '.["id"]' > $FILE
