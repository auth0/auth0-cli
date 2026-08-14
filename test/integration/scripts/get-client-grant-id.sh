#! /bin/bash

client_id=$(./test/integration/scripts/get-m2m-app-id.sh)
audience=$(./test/integration/scripts/get-api-identifier.sh)

auth0 client-grants list --client-id "$client_id" --audience "$audience" --json --no-input | jq -r '.[0].id'
