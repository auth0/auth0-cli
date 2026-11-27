package plugins

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ValidationError reports every problem found in a manifest. Each entry starts
// with the JSON path of the offending field, for example
// "commands[0].permissions.hosts[1].reason".
type ValidationError struct {
	Errors []string
}

// Error returns all problems, one per line.
func (e *ValidationError) Error() string {
	return strings.Join(e.Errors, "\n")
}

// ProblemList collects the problems found by checks that are written in Go
// because JSON Schema cannot express them. A problem is a message that starts
// with the JSON path of the offending field.
type ProblemList []string

// Addf adds a formatted problem.
func (p *ProblemList) Addf(format string, args ...any) {
	*p = append(*p, fmt.Sprintf(format, args...))
}

// CheckUnique adds a problem when value was already seen on another item. The
// seen map holds the path of the first item that used each value. The problem
// is reported on the field of the item at path, with what naming the thing that
// must be unique.
func (p *ProblemList) CheckUnique(seen map[string]string, value, path, field, what string) {
	if first, duplicate := seen[value]; duplicate {
		p.Addf("%s.%s: duplicate %s %q, already used by %s", path, field, what, value, first)
		return
	}

	seen[value] = path
}

// CheckTimeout reports why value is not a timeout, which is a Go duration
// greater than zero such as "30s" or "10m". It returns nil for a timeout.
func CheckTimeout(value string) error {
	if timeout, err := time.ParseDuration(value); err != nil || timeout <= 0 {
		return fmt.Errorf("%q must be a Go duration greater than zero, for example \"30s\" or \"10m\"", value)
	}

	return nil
}

// itemPath returns the path of the item at index in the list field of parent,
// for example "commands[0].flags[1]". An empty parent means the manifest root.
func itemPath(parent, list string, index int) string {
	if parent == "" {
		return fmt.Sprintf("%s[%d]", list, index)
	}

	return fmt.Sprintf("%s.%s[%d]", parent, list, index)
}

// SchemaProblems reports the problems found by a JSON Schema validation, one
// message per problem, each prefixed with the JSON path of the offending field.
// It returns nil when err is not a schema validation error.
func SchemaProblems(err error) []string {
	var root *jsonschema.ValidationError
	if !errors.As(err, &root) {
		return nil
	}

	printer := message.NewPrinter(language.English)

	var problems []string

	var walk func(err *jsonschema.ValidationError)
	walk = func(err *jsonschema.ValidationError) {
		if len(err.Causes) > 0 {
			for _, cause := range err.Causes {
				walk(cause)
			}
			return
		}

		switch k := err.ErrorKind.(type) {
		case *kind.Required:
			for _, missing := range k.Missing {
				problems = append(problems, fmt.Sprintf("%s: required field is missing", joinPath(err.InstanceLocation, missing)))
			}
		case *kind.AdditionalProperties:
			for _, property := range k.Properties {
				problems = append(problems, fmt.Sprintf("%s: unknown field", joinPath(err.InstanceLocation, property)))
			}
		default:
			problems = append(problems, fmt.Sprintf("%s: %s", joinPath(err.InstanceLocation), k.LocalizedString(printer)))
		}
	}
	walk(root)

	slices.Sort(problems)

	return slices.Compact(problems)
}

// joinPath renders JSON location tokens as a path such as "commands[0].name".
// A token made only of digits is treated as an array index.
func joinPath(location []string, extra ...string) string {
	var path strings.Builder

	for _, token := range append(slices.Clone(location), extra...) {
		if _, err := strconv.ParseUint(token, 10, 64); err == nil {
			path.WriteByte('[')
			path.WriteString(token)
			path.WriteByte(']')
			continue
		}

		if path.Len() > 0 {
			path.WriteByte('.')
		}
		path.WriteString(token)
	}

	if path.Len() == 0 {
		return "(root)"
	}

	return path.String()
}
