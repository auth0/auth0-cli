#! /bin/bash

apps=$( auth0 apps list --json --no-input )
for app in $( printf "%s" "$apps" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${app}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.client_id')
    name=$(_jq '.name')

    if [[ $name = "Default App" ]]
    then
        echo $id
        exit 0
    fi
    exit 1
done