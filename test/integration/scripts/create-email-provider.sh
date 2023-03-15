EXIT_STATUS=0
auth0 api POST "emails/provider" --data '{"name":"mandrill","credentials":{"api_key":"some-api-key"}}' || EXIT_STATUS=$?
auth0 api PATCH "emails/provider" --data '{"enabled":true}' || EXIT_STATUS=$?
exit $EXIT_STATUS