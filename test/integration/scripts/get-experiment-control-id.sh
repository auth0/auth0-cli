#! /bin/bash

# Control (baseline) variation on the experiments-suite flag. Its overrides are
# an empty object, which marks it as the control.
FILE=./test/integration/identifiers/experiment-control-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

FF_ID=$(./test/integration/scripts/get-experiment-flag-id.sh)

control=$( auth0 experimentation feature-flags variations create "$FF_ID" \
  -n "integration-test-exp-control" \
  -o '{}' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$control" | jq -r '.["id"]' > $FILE
cat $FILE
