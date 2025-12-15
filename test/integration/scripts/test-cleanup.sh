#! /bin/bash

function delete_resources {
  local cmd=${1-} name=${2-} field=${3-}

  printf "\r\nGathering resource %ss for %s...\r\n" ${field} ${cmd}
  resources=$( auth0 ${cmd} list --json --no-input | jq -r ".[] | select(.name | test(\"^${name}.+\")) | .${field}" | tr '\n' ' ' )

  if [[ $resources ]]
  then
    echo "Deleting resources for ${cmd}... ${field} (e.g. IDs): ${resources}"
    auth0 ${cmd} delete --force ${resources[*]}
  fi
}

delete_resources "apps" "integration-test-app" "client_id"
delete_resources "apis" "integration-test-api" "id"

printf "\r\nGathering resource user_ids for users...\r\n"
# using the search command since users have no list command
users=$( auth0 users search -q "*"  --json --no-input | jq -r '.[] | select(.name | test("^integration-.+")) | .user_id' | tr '\n' ' ' )
if [[ $users ]]
then
  echo "Deleting resources for users... id (e.g. IDs): ${users}"
  auth0 users delete --force ${users[*]}
fi

delete_resources "roles" "integration-test-role" "id"
delete_resources "rules" "integration-test-rule" "id"
delete_resources "orgs" "integration-test-org" "id"
delete_resources "actions" "integration-test-" "id"
delete_resources "token-exchange" "integration-test-" "id"
delete_resources "event-streams" "integration-test-" "id"
delete_resources "logs streams" "integration-test-" "id"

auth0 domains delete $(./test/integration/scripts/get-custom-domain-id.sh) --no-input

# clean up the email provider
auth0 email provider delete --force

# Reset universal login branding
auth0 ul update --accent "#2A2E35" --background "#FF4F40" --logo "https://example.com/logo.png" --favicon "https://example.com/favicon.png" --font https://example.com/font.woff --no-input

# Removes quickstart directory
rm -rf integration-test-app-qs

rm -rf test/integration/identifiers

rm -rdf tmp-tf-gen
