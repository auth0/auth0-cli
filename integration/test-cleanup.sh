#! /bin/bash

apps=$( auth0 apps list --format json --no-input )

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

apis=$( auth0 apis list --format json --no-input )

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
users=$( auth0 users search -q "*"  --format json --no-input )

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

roles=$( auth0 roles list --format json --no-input )

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

rules=$( auth0 rules list --format json --no-input )

for rule in $( echo "${rules}" | jq -r '.[] | @base64' ); do
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
