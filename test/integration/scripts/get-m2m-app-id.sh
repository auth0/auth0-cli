#! /bin/bash

FILE=./test/integration/identifiers/m2m-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

m2m_app=$( auth0 apps create -n integration-test-app-m2m -t m2m --description "M2M test app" --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$m2m_app" | jq -r '.["client_id"]' > $FILE
cat $FILE
