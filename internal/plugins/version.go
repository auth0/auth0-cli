package plugins

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is the MAJOR.MINOR.PATCH core of a semantic version. A prerelease or
// build suffix is not part of it, because the plugin framework ships in beta
// builds of the CLI first and a constraint such as ">=1.39.0" must be met by
// 1.39.0-beta.1.
type Version struct {
	Major, Minor, Patch uint64
}

// ParseVersion reads the core of a semantic version. It accepts a leading "v"
// and ignores a prerelease or build suffix. It reports false for anything that
// does not start with MAJOR.MINOR.PATCH, such as an empty string or "dev".
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(s, "v")

	if end := strings.IndexAny(s, "-+"); end >= 0 {
		s = s[:end]
	}

	return parseCore(s)
}

// isPrerelease reports whether s is a semantic version with a prerelease suffix,
// such as 1.39.0-beta.1 or v1.39.0-rc.2+build. Build metadata alone, as in
// 1.39.0+build, is not a prerelease. Anything that does not start with
// MAJOR.MINOR.PATCH is not a version, so it is not a prerelease either.
func isPrerelease(s string) bool {
	s = strings.TrimPrefix(s, "v")

	end := strings.IndexAny(s, "-+")
	if end < 0 || s[end] != '-' {
		return false
	}

	if _, ok := parseCore(s[:end]); !ok {
		return false
	}

	prerelease, _, _ := strings.Cut(s[end+1:], "+")

	return prerelease != ""
}

// FeatureEnabled reports whether the plugin commands are available in a CLI
// build with the given version. They are for builds made from source, which have
// an empty version or "dev", and for prerelease builds. A stable release carries
// the code but not the feature.
func FeatureEnabled(version string) bool {
	return version == "" || version == "dev" || isPrerelease(version)
}

// parseCore reads exactly MAJOR.MINOR.PATCH, with no leading zeros.
func parseCore(s string) (Version, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}

	var numbers [3]uint64

	for n, part := range parts {
		// ParseUint rejects an empty part, a sign, an underscore, any other
		// character that is not a digit, and a number that overflows.
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil || (len(part) > 1 && part[0] == '0') {
			return Version{}, false
		}

		numbers[n] = number
	}

	return Version{numbers[0], numbers[1], numbers[2]}, true
}

// String returns the version as MAJOR.MINOR.PATCH.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0 or 1 when v is lower than, equal to or higher than
// other.
func (v Version) Compare(other Version) int {
	return cmp.Or(cmp.Compare(v.Major, other.Major), cmp.Compare(v.Minor, other.Minor), cmp.Compare(v.Patch, other.Patch))
}

// errInvalidConstraint means an engines constraint does not follow the grammar.
var errInvalidConstraint = errors.New("invalid engines constraint")

// EnginesSatisfied reports whether the CLI version meets the constraint of the
// engines.auth0-cli field of a plugin, which is one or more space separated
// comparators (>=, <=, >, < or =) followed by MAJOR.MINOR.PATCH that must all
// match.
//
// Only the MAJOR.MINOR.PATCH core of the CLI version is compared, so a prerelease
// suffix is ignored. A CLI version that is not a semantic version, such as the
// "dev" of a local build or an empty string, cannot be compared, so the
// constraint is treated as met. A constraint that does not follow the grammar is
// an error and is never treated as met.
func EnginesSatisfied(constraint, cliVersion string) (bool, error) {
	comparators := strings.Split(constraint, " ")

	type check struct {
		operator string
		version  Version
	}

	checks := make([]check, 0, len(comparators))

	for _, comparator := range comparators {
		var parsed check

		for _, operator := range []string{">=", "<=", ">", "<", "="} {
			if rest, ok := strings.CutPrefix(comparator, operator); ok {
				version, valid := parseCore(rest)
				if !valid {
					return false, fmt.Errorf("%w %q: %q is not MAJOR.MINOR.PATCH", errInvalidConstraint, constraint, rest)
				}

				parsed = check{operator, version}

				break
			}
		}

		if parsed.operator == "" {
			return false, fmt.Errorf("%w %q: %q does not start with >=, <=, >, < or =", errInvalidConstraint, constraint, comparator)
		}

		checks = append(checks, parsed)
	}

	current, ok := ParseVersion(cliVersion)
	if !ok {
		return true, nil
	}

	for _, c := range checks {
		order := current.Compare(c.version)

		var met bool

		switch c.operator {
		case ">=":
			met = order >= 0
		case "<=":
			met = order <= 0
		case ">":
			met = order > 0
		case "<":
			met = order < 0
		case "=":
			met = order == 0
		}

		if !met {
			return false, nil
		}
	}

	return true, nil
}
