#! /bin/bash

json='{"name":"integration-test-rule-newRule","script":"function(user, context, cb) {\n  cb(null, user, context);\n}\n","enabled":false}'
rule=$( echo "$json" | auth0 rules create --json )

mkdir -p ./test/integration/identifiers
echo "$rule" | jq -r '.["id"]' > ./test/integration/identifiers/rule-id
echo "$rule" | jq '.name = "integration-test-rule-betterName"' | jq '.enabled = false' > ./test/integration/fixtures/update-rule.json
