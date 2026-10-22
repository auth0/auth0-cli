#! /bin/bash

# Dedicated feature flag for the experiments suite. Kept separate from
# get-feature-flag-id.sh so the feature-flags suite's flag stays free of
# variations until that suite creates them (its empty-list assertion depends
# on it).
FILE=./test/integration/identifiers/experiment-flag-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

ff=$( auth0 experimentation feature-flags create \
  -n "integration-test-exp-flag" \
  -p '{"color":{"type":"string","value":"blue"}}' \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$ff" | jq -r '.["id"]' > $FILE
cat $FILE
