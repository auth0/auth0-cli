#! /bin/bash

action=$( auth0 actions create -n "integration-test-action" -t "post-login" -c "function() {}" -d "lodash=4.0.0" -s "SECRET=value" --json )

mkdir -p ./test/integration/identifiers
echo "$action" | jq -r '.["id"]' > ./test/integration/identifiers/action-id
