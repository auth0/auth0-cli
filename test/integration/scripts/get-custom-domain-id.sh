#! /bin/bash

testing_domain_name="custom-domain.com"

domains=$( auth0 domains list --json --no-input )
for domain in $( printf "%s" "$domains" | jq -r '.[] | @base64' ); do
    _jq() {
     echo "${domain}" | base64 --decode | jq -r "${1}"
    }

    id=$(_jq '.custom_domain_id')
    name=$(_jq '.domain')

    if [[ $name = $testing_domain_name ]]
    then
        echo $id
        exit 0
    fi
    exit 1
done