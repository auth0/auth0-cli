package validators

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/auth0.v5/management"
)

// TriggerID checks that the provided trigger is valid.
func TriggerID(trigger string) error {
	allTriggerIDs := []string{
		string(management.PostLogin),
		string(management.ClientCredentials),
		string(management.PreUserRegistration),
		string(management.PostUserRegistration),
	}

	for _, id := range allTriggerIDs {
		if trigger == id {
			return nil
		}
	}

	return fmt.Errorf("%s is not a valid trigger type (%s)", trigger, strings.Join(allTriggerIDs, ", "))
}

var semverRe = regexp.MustCompile("^(?P<major>0|[1-9]\\d*)\\.(?P<minor>0|[1-9]\\d*)\\.(?P<patch>0|[1-9]\\d*)(?:-(?P<prerelease>(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?$\n")

// Dependencies parses a slice of dependencies of the format <name>@<semver> and returns them in their
// management.Dependency form.
func Dependencies(deps []string) ([]management.Dependency, error) {
	dependencies := make([]management.Dependency, 0)
	for _, dep := range deps {
		name, version, err := splitNameVersion(dep)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, management.Dependency{
			Name:    name,
			Version: version,
		})
	}
	return dependencies, nil
}

func splitNameVersion(val string) (string, string, error) {
	parts := strings.SplitN(val, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("dependency %s missing version", val)
	}

	name, version := parts[0], parts[1]

	if !semverRe.MatchString(version) {
		return "", "", fmt.Errorf("invalid semver %s for dependency %s", name, version)
	}

	return name, version, nil
}
