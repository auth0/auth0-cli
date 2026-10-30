package cli

import (
	_ "embed"
)

// universalPortalsScript is the Universal Portals setup and audit tool, built into one file that
// Node.js can run. It is generated from the sources in data/universal-portals/_source.
//
//go:embed data/universal-portals/universal-portals.mjs
var universalPortalsScript []byte
