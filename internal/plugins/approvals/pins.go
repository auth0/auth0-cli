package approvals

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// PinsEnv is the environment variable that approves plugins for CI. It holds a
// comma separated list of registry/name@sha256:<hash> entries.
const PinsEnv = "AUTH0_CLI_PLUGIN_APPROVALS"

// pinPattern is the form of one entry, a plugin ID and a consent hash.
var pinPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*)@(sha256:[0-9a-f]{64})$`)

// Pins maps a plugin ID to the consent hashes that are approved for it on every
// tenant. A pin is never written to approvals.json.
type Pins map[string][]string

// ParsePins parses the value of PinsEnv. Spaces around an entry are ignored, an
// empty value means there are no pins, and a plugin ID can be pinned to more
// than one hash. An entry in any other form, such as an empty one after a
// trailing comma, is an error that names it: a typo must never look like having
// no pins.
func ParsePins(value string) (Pins, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	pins := Pins{}

	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)

		match := pinPattern.FindStringSubmatch(entry)
		if match == nil {
			return nil, fmt.Errorf("%w (the entry %q is not in the form registry/name@sha256:<64 lowercase hex digits>)", ErrBadPins, entry)
		}

		if !slices.Contains(pins[match[1]], match[2]) {
			pins[match[1]] = append(pins[match[1]], match[2])
		}
	}

	return pins, nil
}

// approves reports whether the plugin is pinned to the hash.
func (p Pins) approves(pluginID, hash string) bool {
	return slices.Contains(p[pluginID], hash)
}
