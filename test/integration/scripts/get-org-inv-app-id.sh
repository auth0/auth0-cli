#! /bin/bash

FILE=./test/integration/identifiers/org-inv-app-id
if [ -f "$FILE" ]; then
    cat $FILE
    exit 0
fi

app=$( auth0 apps create -n integration-test-app-org-inv -t native --description NewApp --json --no-input )

client_id=$( echo "$app" | jq -r '.["client_id"]' )

# Enable organization support to allow the app to be used for org invitations
auth0 api patch "clients/${client_id}" --data '{"organization_usage":"allow","initiate_login_uri":"https://example.com/login"}' > /dev/null

mkdir -p ./test/integration/identifiers
echo "$client_id" > $FILE
cat $FILE
