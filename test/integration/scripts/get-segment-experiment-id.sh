#! /bin/bash

# A segment-strategy experiment: traffic is routed by segment membership rather
# than by percentage weights. The control allocation is the fallback (served to
# users who match no segment); the treatment allocation targets a segment.
FILE=./test/integration/identifiers/segment-experiment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

FF_ID=$(./test/integration/scripts/get-experiment-flag-id.sh)
CONTROL_ID=$(./test/integration/scripts/get-experiment-control-id.sh)
TREATMENT_ID=$(./test/integration/scripts/get-experiment-treatment-id.sh)
SEGMENT_ID=$(./test/integration/scripts/get-experiment-segment-id.sh)

experiment=$( auth0 experimentation experiments create \
  -n "integration-test-seg-experiment" \
  -f "$FF_ID" \
  -a "authentication" \
  -s "segment" \
  --allocations "[{\"variation_id\":\"$CONTROL_ID\",\"is_control\":true,\"is_fallback\":true},{\"variation_id\":\"$TREATMENT_ID\",\"segment_id\":\"$SEGMENT_ID\",\"priority\":1,\"is_control\":false}]" \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$experiment" | jq -r '.["id"]' > $FILE
cat $FILE
