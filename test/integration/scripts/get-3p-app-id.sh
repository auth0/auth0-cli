#! /bin/bash

FILE=./test/integration/identifiers/3p-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

app=$( auth0 apps create -n integration-test-app-3p-strict -t regular --description 3PApp1 --is-first-party=false --third-party-security-mode strict --redirection-policy open_redirect_protection --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$app" | jq -r '.["client_id"]' > $FILE
cat $FILE
