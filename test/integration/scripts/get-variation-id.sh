#! /bin/bash

# Returns the id of the control variation the feature-flags suite creates on its
# flag (get-feature-flag-id.sh). Looks it up by name rather than creating one,
# so the suite never ends up with two baseline variations on the same flag.
FILE=./test/integration/identifiers/variation-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

FF_ID=$(./test/integration/scripts/get-feature-flag-id.sh)

vid=$( auth0 experimentation feature-flags variations list "$FF_ID" --json --no-input \
  | jq -r '.[] | select(.name == "integration-test-control") | .id' | head -n1 )

mkdir -p ./test/integration/identifiers
echo "$vid" > $FILE
cat $FILE
