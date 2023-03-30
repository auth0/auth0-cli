#! /bin/bash

apis=$( auth0 apis list --json --no-input )
for api in $( printf "%s" "$apis" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${api}" | base64 --decode | jq -r "${1}"
    }

    audience=$(_jq '.identifier')
    name=$(_jq '.name')

    if [[ $name = "Auth0 Management API" ]]
    then
        echo $audience
        exit 0
    fi
    exit 1
done