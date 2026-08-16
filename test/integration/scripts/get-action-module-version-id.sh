#! /bin/bash

FILE=./test/integration/identifiers/action-module-version-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

module_id=$( ./test/integration/scripts/get-action-module-versions-module-id.sh )

# Ensure the module has at least one published version to inspect.
auth0 actions modules versions publish "$module_id" >/dev/null 2>&1

version=$( auth0 actions modules versions list "$module_id" --json )

mkdir -p ./test/integration/identifiers
echo "$version" | jq -r '.[0]["id"]' > $FILE
cat $FILE
