#! /bin/bash

apps=$( auth0 apps list --format json --no-input)

for app in $( echo "${apps}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${app}" | base64 --decode | jq -r "${1}"
    }

    clientid=$(_jq '.ClientID')
    name=$(_jq '.Name')
		# TODO(jfatta): should remove only those 
		# created during the same test session
    if [[ $name = integration-test-* ]]
    then
        echo deleting "$name"
        $( auth0 apps delete "$clientid")
    fi
done
