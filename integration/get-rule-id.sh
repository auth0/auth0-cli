#! /bin/bash

json='{"name":"integration-test-rule-newRule","script":"function(user, context, cb) {\n  cb(null, user, context);\n}\n","enabled":false}'
rule=$( echo "$json" | auth0 rules create --format json )

mkdir -p ./integration/identifiers
echo "$rule" | jq -r '.["id"]' > ./integration/identifiers/rule-id
echo "$rule" | jq '.name = "integration-test-rule-betterName"' | jq '.enabled = false' > ./integration/fixtures/update-rule.json
