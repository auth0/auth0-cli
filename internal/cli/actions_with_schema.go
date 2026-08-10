package cli

import (
	"encoding/json"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// createActionFromJSON creates an action from --data input.
// The JSON is validated against the OpenAPI schema before the API call.
func createActionFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	handler, err := NewInputJSONHandler(cli)
	if err != nil {
		return fmt.Errorf("failed to initialize JSON handler: %w", err)
	}

	// Parse and validate JSON against the schema.
	var rawData map[string]interface{}
	if err := handler.ParseAndValidate(dataStr, "POST", "/actions/actions", &rawData); err != nil {
		cli.renderer.Infof("Run 'auth0 actions create --schema' to see the expected schema.")
		return err
	}

	// Convert to management.Action.
	action := &management.Action{}
	jsonBytes, err := json.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to process JSON input: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, action); err != nil {
		return fmt.Errorf("failed to convert JSON to action: %w", err)
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

// updateActionFromJSON updates an action from --data input.
// The JSON is validated against the OpenAPI schema before the API call.
func updateActionFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	handler, err := NewInputJSONHandler(cli)
	if err != nil {
		return fmt.Errorf("failed to initialize JSON handler: %w", err)
	}

	// Parse and validate JSON against the schema.
	var rawData map[string]interface{}
	path := fmt.Sprintf("/actions/actions/%s", id)
	if err := handler.ParseAndValidate(dataStr, "PATCH", path, &rawData); err != nil {
		cli.renderer.Infof("Run 'auth0 actions update --schema' to see the expected schema.")
		return err
	}

	// Read the existing action to preserve supported_triggers.
	var oldAction *management.Action
	if err := ansi.Waiting(func() (err error) {
		oldAction, err = cli.api.Action.Read(cmd.Context(), id)
		return err
	}); err != nil {
		return fmt.Errorf("failed to read action with ID %q: %w", id, err)
	}

	// Convert to management.Action, preserving triggers.
	updatedAction := &management.Action{
		SupportedTriggers: oldAction.SupportedTriggers,
	}
	jsonBytes, err := json.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to process JSON input: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, updatedAction); err != nil {
		return fmt.Errorf("failed to convert JSON to action: %w", err)
	}

	if err := ansi.Waiting(func() error {
		return cli.api.Action.Update(cmd.Context(), id, updatedAction)
	}); err != nil {
		err = enhanceAPIError(err, "PATCH", path)
		return fmt.Errorf("failed to update action with ID %q: %w", id, err)
	}

	cli.renderer.ActionUpdate(updatedAction)

	return nil
}
