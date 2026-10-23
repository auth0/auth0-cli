// Package runner starts an installed plugin: it separates the flags the CLI owns
// from the arguments of the plugin, builds the small environment the plugin
// gets, and runs the executable with a timeout, stopping the whole process tree
// when the time is up or the caller gives up.
package runner

import (
	"fmt"
	"strconv"
	"strings"
)

// OwnedFlag is a flag that belongs to the CLI and is removed from the arguments
// before they reach the plugin.
type OwnedFlag struct {
	// Name is the long name without the leading dashes, for example "tenant".
	Name string

	// Short is the one letter name without the leading dash, or empty.
	Short string

	// Bool is true for a flag that takes no value, or the optional "=true" or
	// "=false". A bool flag never takes the next argument as its value.
	Bool bool
}

// Scanned is the result of Scan.
type Scanned struct {
	// Values holds the value of each owned flag that was found, by flag name. The
	// value of a bool flag is "true" or "false". A flag that was repeated holds the
	// last value.
	Values map[string]string

	// Rest is every argument that was not removed, in the original order.
	Rest []string

	// Help is true when -h or --help was found.
	Help bool
}

// helpName is the long name of the help flag, which Scan always owns.
const helpName = "help"

// Scan removes the owned flags from args, and leaves everything else untouched.
// Unknown flags, and the arguments that follow them, stay in Rest in their
// original order. It never reads a flag that belongs to the plugin.
//
// The accepted forms are --name, --name=value, --name value for a flag that takes
// a value, -x, -x=value and -x value, and for a bool flag --name or --name=true
// and --name=false. The next argument is taken as the value of a flag that takes
// one even when it starts with a dash, as cobra does, except for a bare "--". A
// short flag is only recognized on its own, so a cluster such as -ab or a value
// glued to the flag such as -xvalue stays in Rest.
//
// A bare "--" stops the scan: it and every argument after it stay in Rest
// unchanged, so the plugin sees them exactly as typed. -h and --help are always
// owned and set Help.
//
// A flag that takes a value but has none, and a bool flag whose value is not a
// bool, are errors.
func Scan(args []string, owned []OwnedFlag) (Scanned, error) {
	result := Scanned{Values: map[string]string{}, Rest: make([]string, 0, len(args))}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			result.Rest = append(result.Rest, args[i:]...)

			break
		}

		name, value, hasValue, ok := split(arg)
		if !ok {
			result.Rest = append(result.Rest, arg)

			continue
		}

		flag, isHelp, found := lookup(owned, name, strings.HasPrefix(arg, "--"))
		if !found {
			result.Rest = append(result.Rest, arg)

			continue
		}

		if flag.Bool || isHelp {
			parsed := "true"

			if hasValue {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return Scanned{}, fmt.Errorf("invalid value %q for %s: use true or false", value, dashed(arg))
				}

				parsed = strconv.FormatBool(b)
			}

			if isHelp {
				result.Help = parsed == "true"
			} else {
				result.Values[flag.Name] = parsed
			}

			continue
		}

		if !hasValue {
			if i+1 >= len(args) || args[i+1] == "--" {
				return Scanned{}, fmt.Errorf("the flag %s needs a value", dashed(arg))
			}

			i++
			value = args[i]
		}

		result.Values[flag.Name] = value
	}

	return result, nil
}

// split breaks an argument that looks like a flag into its name and optional
// value, and reports false for anything else, such as a value, a lone dash or
// "--".
func split(arg string) (name, value string, hasValue, ok bool) {
	switch {
	case strings.HasPrefix(arg, "--"):
		arg = arg[2:]
	case strings.HasPrefix(arg, "-"):
		arg = arg[1:]
	default:
		return "", "", false, false
	}

	if arg == "" {
		return "", "", false, false
	}

	name, value, hasValue = strings.Cut(arg, "=")

	return name, value, hasValue, name != ""
}

// lookup finds the owned flag that a name refers to. The help flag is built in.
func lookup(owned []OwnedFlag, name string, long bool) (flag OwnedFlag, isHelp, found bool) {
	if (long && name == helpName) || (!long && name == "h") {
		return OwnedFlag{}, true, true
	}

	for _, candidate := range owned {
		if long && candidate.Name == name {
			return candidate, false, true
		}

		if !long && candidate.Short != "" && candidate.Short == name {
			return candidate, false, true
		}
	}

	return OwnedFlag{}, false, false
}

// dashed returns the flag of an argument as it was typed, without a value.
func dashed(arg string) string {
	name, _, _ := strings.Cut(arg, "=")

	return name
}
