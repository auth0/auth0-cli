#! /bin/bash

FILE=./test/integration/identifiers/variation-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

# Ensure the feature flag exists first.
FF_ID=$(./test/integration/scripts/get-feature-flag-id.sh)

variation=$( auth0 feature-flags variations create "$FF_ID" \
  -n "integration-test-control" \
  -o '{"color":{"value":"blue"}}' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$variation" | jq -r '.["id"]' > $FILE
cat $FILE
