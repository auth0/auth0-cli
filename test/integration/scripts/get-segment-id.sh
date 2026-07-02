#! /bin/bash

FILE=./test/integration/identifiers/segment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

segment=$( auth0 experimentation segments create \
  -n "integration-test-segment" \
  -r '[{"match":{"connection":["MyConn"]}}]' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$segment" | jq -r '.["id"]' > $FILE
cat $FILE
