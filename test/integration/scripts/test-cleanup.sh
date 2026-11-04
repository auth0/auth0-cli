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
delete_resources "actions modules" "integration-test-module" "id"
delete_resources "token-exchange" "integration-test-" "id"
delete_resources "event-streams" "integration-test-" "id"
delete_resources "forms" "integration-test-" "id"
delete_resources "connections" "integration-test-" "id"
delete_resources "flows vault connections" "integration-test-" "id"
delete_resources "flows" "integration-test-" "id"
delete_resources "logs streams" "integration-test-" "id"

# Experimentation resources need bespoke cleanup: the delete commands accept a
# single ID (not a space-separated list), and an active experiment must be moved
# out of the active state before it can be deleted. Order matters — experiments
# reference feature flags/variations, so they go first; segments are independent.
function delete_experimentation {
  local resource=${1-} name=${2-}

  printf "\r\nGathering experimentation %s for cleanup...\r\n" "${resource}"
  ids=$( auth0 experimentation ${resource} list --json --no-input | jq -r ".[] | select(.name | test(\"^${name}.+\")) | .id" )

  for id in ${ids}; do
    if [[ "${resource}" == "experiments" ]]; then
      # A running experiment cannot be deleted; pause it first (best effort).
      auth0 experimentation experiments update "${id}" --status paused --no-input >/dev/null 2>&1 || true
    fi
    echo "Deleting experimentation ${resource}: ${id}"
    auth0 experimentation ${resource} delete "${id}" --force
  done
}

delete_experimentation "experiments" "integration-test-"
delete_experimentation "feature-flags" "integration-test-"
delete_experimentation "segments" "integration-test-"

auth0 domains delete $(./test/integration/scripts/get-custom-domain-id.sh) --no-input

# clean up the email provider
auth0 email provider delete --force

# Reset universal login branding
auth0 ul update --accent "#2A2E35" --background "#FF4F40" --logo "https://example.com/logo.png" --favicon "https://example.com/favicon.png" --font https://example.com/font.woff --no-input

# Removes quickstart directory
rm -rf integration-test-app-qs

rm -rf test/integration/identifiers

rm -rdf tmp-tf-gen
