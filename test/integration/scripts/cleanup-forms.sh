#!/bin/bash

set -euo pipefail

ids=()
while IFS= read -r id; do
  if [[ -n "$id" ]]; then
    ids+=("$id")
  fi
done < <(auth0 forms list --json --no-input | jq -r '.[] | select(.name | startswith("integration-test-")) | .id')

if (( ${#ids[@]} > 0 )); then
  auth0 forms delete --force "${ids[@]}"
fi

rm -f ./test/integration/identifiers/form-id
