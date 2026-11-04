#! /bin/bash

FILE=./test/integration/identifiers/form-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

form=$( auth0 forms create --name "integration-test-form" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$form" | jq -r '.["id"]' > $FILE
cat $FILE
