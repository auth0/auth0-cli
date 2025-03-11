#! /bin/bash

FILE=./test/integration/identifiers/custom-phone-provider-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

# Create the phone provider.
phone_provider_id=$( auth0 phone provider create -p "custom" --disabled=false --configuration='{"delivery_methods":["text", "voice"]}' --json | jq -r '.["id"]' )

mkdir -p ./test/integration/identifiers
echo "$phone_provider_id" > $FILE
cat $FILE