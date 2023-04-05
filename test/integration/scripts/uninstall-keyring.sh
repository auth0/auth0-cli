#!/bin/bash

# Only run this test on linux
if which apt-get >/dev/null 2>&1; then
   sudo apt-get purge dbus-x11 gnome-keyring # Uninstall keyring

    # go run ./test/integration/scripts/expire-access-token.go
    # auth0 apps list # Expect this to fail

   auth0 logout $AUTH0_CLI_CLIENT_DOMAIN # Logging out will clear-out all saved credentials, necessary 
   auth0 login --domain "${AUTH0_CLI_CLIENT_DOMAIN}" --client-id "${AUTH0_CLI_CLIENT_ID}" --client-secret "${AUTH0_CLI_CLIENT_SECRET}"
fi