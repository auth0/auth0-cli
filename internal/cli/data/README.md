# Embedded data

This directory contains files that get embedded into the CLI binary.

## Contents

### Rule templates
A series of JS files prefixed with `rule-template-`. Each file contains a single rule that can be used as a code template when creating a new rule with `auth0 rules create`.

### Action templates
A series of JS files prefixed with `action-template-`, each containing an empty action for a given extensibility point. There's an empty action for each extensibility point. These are used as code templates for new actions created with `auth0 actions create`.
