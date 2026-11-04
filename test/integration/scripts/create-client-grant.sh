#! /bin/bash

management_api_audience=$(./test/integration/scripts/get-manage-api-audience.sh)
m2m_client_id=$(./test/integration/scripts/get-m2m-app-id.sh)

auth0 client-grants create --client-id "$m2m_client_id" --audience "$management_api_audience" --no-input