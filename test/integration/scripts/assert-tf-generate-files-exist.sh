#!/bin/bash

if [[ $1 == *partial_success* ]]; then
    files=(
        "tmp-tf-gen/auth0_import.tf" 
        "tmp-tf-gen/auth0_main.tf" 
    )
else
    files=(
        "tmp-tf-gen/auth0_generated.tf" 
        "tmp-tf-gen/auth0_import.tf" 
        "tmp-tf-gen/auth0_main.tf" 
        "tmp-tf-gen/terraform" 
        "tmp-tf-gen/.terraform.lock.hcl" 
    )
fi

has_error=false

for file in "${files[@]}"; do
    if [ -e "$file" ]; then
        if ! [ -s "$file" ]; then
        echo "$file exists but is empty"
            has_error=true
        fi
    else
        echo "$file does not exist"
        has_error=true
    fi
done

if [ "$has_error" = true ]; then
    exit 1
fi