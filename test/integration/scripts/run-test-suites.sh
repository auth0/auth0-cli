#! /bin/bash -v

set -e

if [[ -z "${AUTH0_DOMAIN}" || -z "${AUTH0_CLIENT_ID}" || -z "${AUTH0_CLIENT_SECRET}"  ]]; then
   echo "Error: AUTH0_DOMAIN, AUTH0_CLIENT_ID and AUTH0_CLIENT_SECRET environment variables need to be set"
   exit 1
fi

auth0 login \
   --domain "${AUTH0_DOMAIN}" \
   --client-id "${AUTH0_CLIENT_ID}" \
   --client-secret "${AUTH0_CLIENT_SECRET}"

set +e

exit_code=0
if [[ -n "${FILE}" ]]; then
   commander test --filter "$FILTER" "${FILE}"
   if [[ $? -ne 0 ]]; then
      exit_code=1
   fi
else
   # The quickstart integration tests are excluded from the default suite, so run
   # each remaining test-cases file individually (in alphabetical order, matching
   # --dir) instead of the whole directory.
   for suite in ./test/integration/*.yaml; do
      if [[ "$(basename "$suite")" == "quickstarts-test-cases.yaml" ]]; then
         echo "Skipping $suite"
         continue
      fi

      commander test --filter "$FILTER" "$suite"
      if [[ $? -ne 0 ]]; then
         exit_code=1
      fi
   done
fi

bash ./test/integration/scripts/test-cleanup.sh

auth0 logout "$AUTH0_DOMAIN"

exit $exit_code
