#! /bin/bash

FILE=./test/integration/identifiers/custom-phone-action-id
if [ -e "$FILE" ]
then
	action_id=$(cat $FILE)

	# Delete the action.
	auth0 actions delete "$action_id" --force

	rm "$FILE"
fi

