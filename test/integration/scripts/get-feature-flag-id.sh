#! /bin/bash

FILE=./test/integration/identifiers/feature-flag-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

ff=$( auth0 feature-flags create \
  -n "integration-test-flag" \
  -p '{"color":{"type":"string","value":"blue"}}' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$ff" | jq -r '.["id"]' > $FILE
cat $FILE
