#! /bin/bash

# Treatment variation on the experiments-suite flag. Its override value must
# differ from the flag's parameter default (blue), so use red.
FILE=./test/integration/identifiers/experiment-treatment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

FF_ID=$(./test/integration/scripts/get-experiment-flag-id.sh)

treatment=$( auth0 experimentation feature-flags variations create "$FF_ID" \
  -n "integration-test-exp-treatment" \
  -o '{"color":"red"}' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$treatment" | jq -r '.["id"]' > $FILE
cat $FILE
