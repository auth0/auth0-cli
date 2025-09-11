#!/bin/bash

FILE=./test/integration/identifiers/network-acl-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

acl=$( auth0 network-acl create --description "integration-test-acl" --active false --priority 9 --rule '{"action":{"log":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.5/24"]}}' --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$acl" | jq -r '.["id"]' > $FILE
cat $FILE
