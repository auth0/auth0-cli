#! /bin/bash

FILE=./test/integration/identifiers/experiment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

# Reuse the dedicated experiments-suite flag and its control + treatment
# variations for a balanced percentage split.
FF_ID=$(./test/integration/scripts/get-experiment-flag-id.sh)
CONTROL_ID=$(./test/integration/scripts/get-experiment-control-id.sh)
TREATMENT_ID=$(./test/integration/scripts/get-experiment-treatment-id.sh)

# The flag now has 2 variations (control + treatment), so it can be activated.
# An experiment can only transition to active while its feature flag is active,
# so activate it here to support the lifecycle tests. Already-active is a no-op.
auth0 experimentation feature-flags update "$FF_ID" --status active --no-input >/dev/null 2>&1 || true

experiment=$( auth0 experimentation experiments create \
  -n "integration-test-experiment" \
  -f "$FF_ID" \
  -a "authentication" \
  -s "percentage" \
  --allocations "[{\"variation_id\":\"$CONTROL_ID\",\"weight\":50,\"is_control\":true},{\"variation_id\":\"$TREATMENT_ID\",\"weight\":50,\"is_control\":false}]" \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$experiment" | jq -r '.["id"]' > $FILE
cat $FILE
