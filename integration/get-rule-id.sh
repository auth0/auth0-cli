#! /bin/bash

role=$( auth0 rules create -n integration-test-rule-newRule -t "Empty rule" --enabled=false --format json --no-input )

mkdir -p ./integration/identifiers
echo "$rule" | jq -r '.["id"]' > ./integration/identifiers/rule-id
