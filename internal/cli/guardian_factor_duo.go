package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
)

func guardianFactorDuoCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "duo",
		Short: "Manage the Duo multi-factor authentication factor",
		Long:  "Manage the Duo MFA factor settings.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	settings := &cobra.Command{
		Use:   "settings",
		Short: "Manage the Duo settings",
		Long:  "Manage the Duo MFA factor settings.",
	}
	settings.SetUsageTemplate(resourceUsageTemplate())
	settings.AddCommand(showGuardianDuoSettingsCmd(cli))
	settings.AddCommand(setGuardianDuoSettingsCmd(cli))
	settings.AddCommand(updateGuardianDuoSettingsCmd(cli))

	cmd.AddCommand(settings)

	return cmd
}

func guardianDuoSettingsRows(host, ikey, skey string) [][]string {
	return [][]string{
		{"HOST", host},
		{"INTEGRATION KEY", ikey},
		{"SECRET KEY", display.MaskSecret(skey)},
	}
}

func showGuardianDuoSettingsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Show the Duo settings",
		Long:    "Display the Duo MFA factor settings.",
		Example: `  auth0 guardian factors duo settings show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var resp *managementv3.GetGuardianFactorDuoSettingsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorDuo.Get(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read Duo settings: %w", err)
			}

			cli.renderer.GuardianDetail("duo settings", guardianDuoSettingsRows(resp.GetHost(), resp.GetIkey(), resp.GetSkey()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)

	return cmd
}

func setGuardianDuoSettingsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Ikey string
		Skey string
		Host string
	}

	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.NoArgs,
		Short: "Set the Duo settings",
		Long: "Replace the Duo MFA factor settings. This overwrites all Duo settings, so the host, " +
			"integration key and secret key are all required. To change a single field without " +
			"clearing the others, use `auth0 guardian factors duo settings update` instead.",
		Example: `  auth0 guardian factors duo settings set --ikey <ikey> --skey <skey> --host api-xxxx.duosecurity.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianDuoHost.Ask(cmd, &inputs.Host, nil); err != nil {
				return err
			}
			if err := guardianDuoIkey.Ask(cmd, &inputs.Ikey, nil); err != nil {
				return err
			}
			if err := guardianDuoSkey.AskPassword(cmd, &inputs.Skey); err != nil {
				return err
			}

			if inputs.Host == "" || inputs.Ikey == "" || inputs.Skey == "" {
				return fmt.Errorf(
					"--host, --ikey and --skey are all required for set, since it replaces the entire " +
						"Duo configuration. To change a single field, use 'auth0 guardian factors duo settings update'",
				)
			}

			body := &managementv3.SetGuardianFactorDuoSettingsRequestContent{
				Host: &inputs.Host,
				Ikey: &inputs.Ikey,
				Skey: &inputs.Skey,
			}

			var resp *managementv3.SetGuardianFactorDuoSettingsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorDuo.Set(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set Duo settings: %w", err)
			}

			cli.renderer.GuardianDetail("duo settings updated", guardianDuoSettingsRows(resp.GetHost(), resp.GetIkey(), resp.GetSkey()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianDuoIkey.RegisterString(cmd, &inputs.Ikey, "")
	guardianDuoSkey.RegisterString(cmd, &inputs.Skey, "")
	guardianDuoHost.RegisterString(cmd, &inputs.Host, "")

	return cmd
}

func updateGuardianDuoSettingsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Ikey string
		Skey string
		Host string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update the Duo settings",
		Long: "Partially update the Duo MFA factor settings. Only the fields you provide are changed; " +
			"the rest keep their current values. Run without flags to be prompted for each field, " +
			"pre-filled with the current value (leave the secret key blank to keep it unchanged).",
		Example: `  auth0 guardian factors duo settings update --host api-xxxx.duosecurity.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fetch the current settings so the interactive prompts can default
			// to the existing values (the secret key is never pre-filled).
			var current *managementv3.GetGuardianFactorDuoSettingsResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.GuardianFactorDuo.Get(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read Duo settings: %w", err)
			}

			currentHost := current.GetHost()
			currentIkey := current.GetIkey()
			if err := guardianDuoHost.AskU(cmd, &inputs.Host, &currentHost); err != nil {
				return err
			}
			if err := guardianDuoIkey.AskU(cmd, &inputs.Ikey, &currentIkey); err != nil {
				return err
			}
			if err := guardianDuoSkey.AskPasswordU(cmd, &inputs.Skey); err != nil {
				return err
			}

			body := &managementv3.UpdateGuardianFactorDuoSettingsRequestContent{}
			if inputs.Ikey != "" {
				body.Ikey = &inputs.Ikey
			}
			if inputs.Skey != "" {
				body.Skey = &inputs.Skey
			}
			if inputs.Host != "" {
				body.Host = &inputs.Host
			}

			if err := ansi.Waiting(func() (err error) {
				_, err = cli.apiv3.GuardianFactorDuo.Update(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update Duo settings: %w", err)
			}

			// The PATCH response only echoes the fields that were sent, so
			// re-fetch the full settings to render the complete current state.
			var resp *managementv3.GetGuardianFactorDuoSettingsResponseContent
			if err := ansi.Waiting(func() (err error) {
				resp, err = cli.apiv3.GuardianFactorDuo.Get(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read Duo settings: %w", err)
			}

			cli.renderer.GuardianDetail("duo settings updated", guardianDuoSettingsRows(resp.GetHost(), resp.GetIkey(), resp.GetSkey()), resp)

			return nil
		},
	}

	registerGuardianJSONFlags(cli, cmd)
	guardianDuoIkey.RegisterString(cmd, &inputs.Ikey, "")
	guardianDuoSkey.RegisterString(cmd, &inputs.Skey, "")
	guardianDuoHost.RegisterString(cmd, &inputs.Host, "")

	return cmd
}
