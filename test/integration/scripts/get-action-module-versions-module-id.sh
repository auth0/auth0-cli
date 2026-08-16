#! /bin/bash

# The versions suite needs a module of its own. The shared module from
# get-action-module-id.sh is deleted by the actions modules suite, which runs
# first, leaving a stale cached ID behind.
FILE=./test/integration/identifiers/action-module-versions-module-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

module=$( auth0 actions modules create -n "integration-test-module-versions" -c "module.exports = {}" -d "lodash=4.0.0" -s "SECRET=value" --json )

mkdir -p ./test/integration/identifiers
echo "$module" | jq -r '.["id"]' > $FILE
cat $FILE
