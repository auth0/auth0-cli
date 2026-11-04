#! /bin/bash

FILE=./test/integration/identifiers/action-module-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

module=$( auth0 actions modules create -n "integration-test-module" -c "module.exports = {}" -d "lodash=4.0.0" -s "SECRET=value" --json )

mkdir -p ./test/integration/identifiers
echo "$module" | jq -r '.["id"]' > $FILE
cat $FILE
