#! /bin/bash

FILE=./test/integration/identifiers/custom-phone-action-id

# Create the action.
action_id=$( auth0 actions create -n "integration-test-custom-phone-action" -t "custom-phone-provider" -c "exports.onExecuteCustomPhoneProvider = async (event, api) => { return; };" --json | jq -r '.["id"]' )

# Deploy the action.
auth0 actions deploy "$action_id"

mkdir -p ./test/integration/identifiers
echo "$action_id" > $FILE
