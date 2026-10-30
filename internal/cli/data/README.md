# Embedded data

This directory contains files that get embedded into the CLI binary.

## Contents

### Rule templates
A series of JS files prefixed with `rule-template-`. Each file contains a single rule that can be used as a code template when creating a new rule with `auth0 rules create`.

### Action templates
A series of JS files prefixed with `action-template-`, each containing an empty action for a given extensibility point. There's an empty action for each extensibility point. These are used as code templates for new actions created with `auth0 actions create`.

### Universal Portals tool
The `universal-portals` folder holds the Universal Portals setup and audit tool.

- `universal-portals/universal-portals.mjs` is the only file embedded into the binary. It is one JavaScript file that Node.js can run, generated from the sources next to it. Do not edit it by hand.
- `universal-portals/_source` is the TypeScript project the file is built from, with its own tests, `package.json` and lockfile. It is not embedded, and Go tooling skips it because of the leading underscore.

To change the tool, edit `_source/src`, then run `npm test` and `npm run bundle` inside `_source`, and commit the sources together with the rebuilt `universal-portals.mjs`. A test fails when the file is older than its sources.
