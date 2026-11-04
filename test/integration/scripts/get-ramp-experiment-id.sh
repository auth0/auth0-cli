#! /bin/bash

# A ramp experiment: traffic follows the 10 -> 50 -> 100 level schedule, so the
# allocations carry no weights. It runs on mfa_enrollment so it never competes
# with the suite's authentication-flow experiments.
FILE=./test/integration/identifiers/ramp-experiment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

FF_ID=$(./test/integration/scripts/get-experiment-flag-id.sh)
CONTROL_ID=$(./test/integration/scripts/get-experiment-control-id.sh)
TREATMENT_ID=$(./test/integration/scripts/get-experiment-treatment-id.sh)

experiment=$( auth0 experimentation experiments create \
  -n "integration-test-ramp-experiment" \
  -f "$FF_ID" \
  -a "mfa_enrollment" \
  -s "percentage" \
  --levels 10,50,100 \
  --allocations "[{\"variation_id\":\"$CONTROL_ID\",\"is_control\":true},{\"variation_id\":\"$TREATMENT_ID\",\"is_control\":false}]" \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$experiment" | jq -r '.["id"]' > $FILE
cat $FILE
