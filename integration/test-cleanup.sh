#! /bin/bash

# update when escape character bug fixed
output=$( auth0 apps list --format json --no-input)
apps=${output:12}

for app in $( echo "${apps}" | jq -r '.[] | @base64' ); do
    _jq() {
     echo ${app} | base64 --decode | jq -r ${1}
    }

    clientid=$( echo $(_jq '.ClientID') )
    name=$( echo $(_jq '.Name') )

    if [[ $name != "Default App" ]]
    then
        echo deleting $name
        echo $( auth0 apps delete $clientid)
    fi
done
