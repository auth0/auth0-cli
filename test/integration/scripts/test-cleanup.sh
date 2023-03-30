#! /bin/bash

apps=$( auth0 apps list --json --no-input )

for app in $( echo "${apps}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${app}" | base64 --decode | jq -r "${1}"
    }

    clientid=$(_jq '.client_id')
    name=$(_jq '.name')
		# TODO(jfatta): should remove only those 
		# created during the same test session
    if [[ $name = integration-test-app-* ]]
    then
        echo deleting "$name"
        $( auth0 apps delete "$clientid")
    fi
done

apis=$( auth0 apis list --json --no-input )

for api in $( echo "${apis}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${api}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.id')
    name=$(_jq '.name')
		# TODO(jfatta): should remove only those 
		# created during the same test session
    if [[ $name = integration-test-api-* ]]
    then
        echo deleting "$name"
        $( auth0 apis delete "$id")
    fi
done

# using the search command since users have no list command
users=$( auth0 users search -q "*"  --json --no-input )

for user in $( echo "${users}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${user}" | base64 --decode | jq -r "${1}"
    }

    userid=$(_jq '.user_id')
		# created during the same test session
    if [[ integration-* ]]
    then
        echo deleting "$userid"
        $( auth0 users delete "$userid")
    fi
done

roles=$( auth0 roles list --json --no-input )

for role in $( echo "${roles}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${role}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.id')
    name=$(_jq '.name')
		# TODO(jfatta): should remove only those
		# created during the same test session
    if [[ $name = integration-test-role-* ]]
    then
        echo deleting "$name"
        $( auth0 roles delete "$id")
    fi
done

rules=$( auth0 rules list --json --no-input )

for rule in $( printf "%s" "$rules" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${rule}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.id')
    name=$(_jq '.name')
		# TODO(jfatta): should remove only those
		# created during the same test session
    if [[ $name = integration-test-rule-* ]]
    then
        echo deleting "$name"
        $( auth0 rules delete "$id")
    fi
done

orgs=$( auth0 orgs list --json --no-input )

for org in $( echo "${orgs}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${org}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.id')
    name=$(_jq '.name')
    if [[ $name = integration-test-org-* ]]
    then
        echo deleting "$name"
        $( auth0 orgs delete "$id")
    fi
done

actions=$( auth0 actions list --json --no-input )

for action in $( echo "${actions}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${action}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.id')
    name=$(_jq '.name')

    if [[ $name = integration-test-* ]]
    then
        echo deleting "$name"
        $( auth0 actions delete "$id")
    fi
done

# Reset universal login branding
auth0 ul update --accent "#2A2E35" --background "#FF4F40" --logo "https://example.com/logo.png" --favicon "https://example.com/favicon.png" --font https://example.com/font.woff --no-input

rm -rf test/integration/identifiers
