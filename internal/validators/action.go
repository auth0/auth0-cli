package validators

import (
	"fmt"
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
