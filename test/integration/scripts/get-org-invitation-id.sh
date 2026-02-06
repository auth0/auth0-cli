#!/bin/bash

FILE=./test/integration/identifiers/org-invitation-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

org_id=$(./test/integration/scripts/get-org-id.sh)
app_id=$(./test/integration/scripts/get-app-id.sh)

invitation=$( auth0 orgs invitations create "$org_id" \
  --inviter-name "Integration Tester" \
  --invitee-email "test@test.com" \
  --client-id "$app_id" \
  --json --no-input )

mkdir -p ./test/integration/identifiers
echo "$invitation" | jq -r '.["id"]' > $FILE
cat $FILE
