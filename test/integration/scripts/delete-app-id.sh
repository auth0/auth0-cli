#!/bin/bash

// This script is used to delete the app identifier file created during integration tests. It checks if the file exists and removes it if it does.
// This script should be called only if the app is successfully deleted in the test cases, to ensure that the identifier file is cleaned up for subsequent test runs.

FILE=./test/integration/identifiers/app-id

# Remove the app identifier file only if it exists
if [ -f "$FILE" ]; then
    rm "$FILE"
    exit $?
fi

# File doesn't exist, nothing to do
exit 0
