#! /bin/bash

FILE=./test/integration/identifiers/flow-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

flow=$( auth0 flows create --name "integration-test-flow" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$flow" | jq -r '.["id"]' > $FILE
cat $FILE
