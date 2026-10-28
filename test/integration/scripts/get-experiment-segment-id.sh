#! /bin/bash

# Dedicated segment for the experiments suite's segment-strategy experiment.
# Kept separate from get-segment-id.sh, and deleted at the end of the
# experiments suite so the segments suite still starts with an empty list.
FILE=./test/integration/identifiers/experiment-segment-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

segment=$( auth0 experimentation segments create \
  -n "integration-test-exp-segment" \
  -r '[{"match":{"country":["US"]}}]' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$segment" | jq -r '.["id"]' > $FILE
cat $FILE
