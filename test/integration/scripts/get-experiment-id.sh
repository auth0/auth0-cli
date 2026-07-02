#! /bin/bash

FILE=./test/integration/identifiers/experiment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

# Ensure the feature flag and a control variation exist first.
FF_ID=$(./test/integration/scripts/get-feature-flag-id.sh)
CONTROL_ID=$(./test/integration/scripts/get-variation-id.sh)

# Create a second (treatment) variation for the allocation pair.
treatment=$( auth0 experimentation feature-flags variations create "$FF_ID" \
  -n "integration-test-treatment" \
  -o '{"color":{"value":"red"}}' \
  --json --no-input )
TREATMENT_ID=$(echo "$treatment" | jq -r '.["id"]')

experiment=$( auth0 experimentation experiments create \
  -n "integration-test-experiment" \
  -f "$FF_ID" \
  -a "login" \
  -s "percentage" \
  -A "[{\"variation_id\":\"$CONTROL_ID\",\"weight\":50,\"is_control\":true},{\"variation_id\":\"$TREATMENT_ID\",\"weight\":50,\"is_control\":false}]" \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$experiment" | jq -r '.["id"]' > $FILE
cat $FILE
