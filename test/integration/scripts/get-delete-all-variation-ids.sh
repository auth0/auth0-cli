#! /bin/bash

# Returns the space-separated ids of the variations on the delete-all flag
# (get-delete-all-flag-id.sh), creating the flag first if needed.
./test/integration/scripts/get-delete-all-flag-id.sh > /dev/null
cat ./test/integration/identifiers/delete-all-variation-ids
