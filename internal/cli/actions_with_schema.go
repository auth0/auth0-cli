package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// createActionFromJSON creates an action from a JSON payload, validated against
// the OpenAPI schema before the API call.
func createActionFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	handler, err := NewDataJSONHandler(cli)
	if err != nil {
		return fmt.Errorf("failed to initialize JSON handler: %w", err)
	}

	action := &management.Action{}
	if err := handler.ParseAndValidate(dataStr, "POST", "/actions/actions", action); err != nil {
		cli.renderer.Infof("Run 'auth0 actions create --schema' to see the expected schema.")
		return err
	}

	if err := ansi.Waiting(func() error {
		return cli.api.Action.Create(cmd.Context(), action)
	}); err != nil {
		err = enhanceAPIError(err, "POST", "/actions/actions")
		return fmt.Errorf("failed to create action: %w", err)
	}

	cli.renderer.ActionCreate(action)

	return nil
}

// updateActionFromJSON updates an action from a JSON payload, validated against
// the OpenAPI schema before the API call.
func updateActionFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	handler, err := NewDataJSONHandler(cli)
	if err != nil {
		return fmt.Errorf("failed to initialize JSON handler: %w", err)
	}

	// Validate against the templated path: the OpenAPI doc keys this operation as
	// /actions/actions/{id}, and kin-openapi matches only when template-variable
	// counts are equal, so a concrete ID would never resolve.
	const schemaPath = "/actions/actions/{id}"
	updatedAction := &management.Action{}
	if err := handler.ParseAndValidate(dataStr, "PATCH", schemaPath, updatedAction); err != nil {
		cli.renderer.Infof("Run 'auth0 actions update --schema' to see the expected schema.")
		return err
	}

	// Name and SupportedTriggers have no `omitempty`, so leaving them unset would
	// send "name": null / "supported_triggers": null and clobber the action.
	// Backfill them from the existing action when the payload omits them.
	if updatedAction.Name == nil || updatedAction.SupportedTriggers == nil {
		var oldAction *management.Action
		if err := ansi.Waiting(func() (err error) {
			oldAction, err = cli.api.Action.Read(cmd.Context(), id)
			return err
		}); err != nil {
			return fmt.Errorf("failed to read action with ID %q: %w", id, err)
		}
		if updatedAction.Name == nil {
			updatedAction.Name = oldAction.Name
		}
		if updatedAction.SupportedTriggers == nil {
			updatedAction.SupportedTriggers = oldAction.SupportedTriggers
		}
	}

	if err := ansi.Waiting(func() error {
		return cli.api.Action.Update(cmd.Context(), id, updatedAction)
	}); err != nil {
		err = enhanceAPIError(err, "PATCH", schemaPath)
		return fmt.Errorf("failed to update action with ID %q: %w", id, err)
	}

	cli.renderer.ActionUpdate(updatedAction)

	return nil
}
