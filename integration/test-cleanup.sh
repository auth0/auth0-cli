#! /bin/bash

apps=$( auth0 apps list --format json --no-input )

for app in $( echo "${apps}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${app}" | base64 --decode | jq -r "${1}"
    }

    clientid=$(_jq '.ClientID')
    name=$(_jq '.Name')
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

    id=$(_jq '.ID')
    name=$(_jq '.Name')
		# TODO(jfatta): should remove only those 
		# created during the same test session
    if [[ $name = integration-test-api-* ]]
    then
        echo deleting "$name"
        $( auth0 apis delete "$id")
    fi
done
