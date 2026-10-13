#! /bin/bash

# Returns the id of a dedicated flag used to exercise deleting every variation.
# Created with three variations (ids stored in delete-all-variation-ids, space
# separated) so the suite can delete them in a single multi-ID command.
FILE=./test/integration/identifiers/delete-all-flag-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

ff=$( auth0 experimentation feature-flags create \
  -n "integration-test-ff-delete-all" \
  -p '{"color":{"type":"string","value":"blue"}}' \
  --json --no-input | jq -r '.["id"]' )

ids=()
for n in one two three; do
  ids+=("$( auth0 experimentation feature-flags variations create "$ff" \
    -n "integration-test-var-$n" -o "{\"color\":\"$n\"}" \
    --json --no-input | jq -r '.["id"]' )")
done

mkdir -p ./test/integration/identifiers
echo "${ids[*]}" > ./test/integration/identifiers/delete-all-variation-ids
echo "$ff" > $FILE
cat $FILE
