package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	guardianEnrollmentID = Argument{
		Name: "Id",
		Help: "Id of the enrollment.",
	}
	guardianEnrollmentUserID = Flag{
		Name:       "User ID",
		LongForm:   "user-id",
		ShortForm:  "u",
		Help:       "User ID to create the enrollment ticket for.",
		IsRequired: true,
	}
	guardianEnrollmentFactor = Flag{
		Name:      "Factor",
		LongForm:  "factor",
		ShortForm: "f",
		Help:      "Factor the user must enroll with, e.g. push-notification, sms, email, otp, webauthn-roaming, webauthn-platform, recovery-code, duo.",
	}
	guardianEnrollmentEmail = Flag{
		Name:     "Email",
		LongForm: "email",
		Help:     "Alternate email address to send the enrollment email to. Defaults to the user's email.",
	}
	guardianEnrollmentSendEmail = Flag{
		Name:     "Send Email",
		LongForm: "send-email",
		Help:     "Send an email to the user to start the enrollment.",
	}
	guardianEnrollmentEmailLocale = Flag{
		Name:     "Email Locale",
		LongForm: "email-locale",
		Help:     "Locale of the enrollment email. Used with --send-email.",
	}
	guardianEnrollmentAllowMultiple = Flag{
		Name:     "Allow Multiple Enrollments",
		LongForm: "allow-multiple",
		Help:     "Allow a user who has previously enrolled in MFA to enroll with additional factors. Universal Login only.",
	}
)

func guardianEnrollmentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrollments",
		Short: "Manage multi-factor authentication enrollments",
		Long:  "Manage user multi-factor authentication (MFA) enrollments and enrollment tickets.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(createGuardianEnrollmentTicketCmd(cli))
	cmd.AddCommand(showGuardianEnrollmentCmd(cli))
	cmd.AddCommand(deleteGuardianEnrollmentCmd(cli))

	return cmd
}

func createGuardianEnrollmentTicketCmd(cli *cli) *cobra.Command {
	var inputs struct {
		UserID        string
		Factor        string
		Email         string
		SendEmail     bool
		EmailLocale   string
		AllowMultiple bool
	}

	cmd := &cobra.Command{
		Use:   "create-ticket",
		Args:  cobra.NoArgs,
		Short: "Create a multi-factor authentication enrollment ticket",
		Long: "Create an MFA enrollment ticket for a user and, optionally, email it to them.\n\n" +
			"The returned ticket URL is the link the user follows to enroll.",
		Example: `  auth0 guardian enrollments create-ticket --user-id "auth0|123"
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --factor push-notification
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --send-email --email me@example.com
  auth0 guardian enrollments create-ticket --user-id "auth0|123" --allow-multiple --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := guardianEnrollmentUserID.Ask(cmd, &inputs.UserID, nil); err != nil {
				return err
			}

			body := &managementv3.CreateGuardianEnrollmentTicketRequestContent{
				UserID: inputs.UserID,
			}
			if inputs.Factor != "" {
				factor, err := managementv3.NewGuardianEnrollmentFactorEnumFromString(inputs.Factor)
				if err != nil {
					return fmt.Errorf("invalid factor %q: %w", inputs.Factor, err)
				}
				body.Factor = &factor
			}
			if inputs.Email != "" {
				body.Email = &inputs.Email
			}
			if guardianEnrollmentSendEmail.IsSet(cmd) {
				body.SendMail = &inputs.SendEmail
			}
			if inputs.EmailLocale != "" {
				body.EmailLocale = &inputs.EmailLocale
			}
			if guardianEnrollmentAllowMultiple.IsSet(cmd) {
				body.AllowMultipleEnrollments = &inputs.AllowMultiple
			}

			var ticket *managementv3.CreateGuardianEnrollmentTicketResponseContent
			if err := ansi.Waiting(func() (err error) {
				ticket, err = cli.apiv3.GuardianEnrollment.CreateTicket(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create guardian enrollment ticket for user %q: %w", inputs.UserID, err)
			}

			cli.renderer.GuardianEnrollmentTicketCreate(ticket)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	guardianEnrollmentUserID.RegisterString(cmd, &inputs.UserID, "")
	guardianEnrollmentFactor.RegisterString(cmd, &inputs.Factor, "")
	guardianEnrollmentEmail.RegisterString(cmd, &inputs.Email, "")
	guardianEnrollmentSendEmail.RegisterBool(cmd, &inputs.SendEmail, false)
	guardianEnrollmentEmailLocale.RegisterString(cmd, &inputs.EmailLocale, "")
	guardianEnrollmentAllowMultiple.RegisterBool(cmd, &inputs.AllowMultiple, false)

	return cmd
}

func showGuardianEnrollmentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a multi-factor authentication enrollment",
		Long:  "Display the status, type and details of an MFA enrollment.",
		Example: `  auth0 guardian enrollments show <enrollment-id>
  auth0 guardian enrollments show <enrollment-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := guardianEnrollmentID.Ask(cmd, &inputs.ID); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var enrollment *managementv3.GetGuardianEnrollmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				enrollment, err = cli.apiv3.GuardianEnrollment.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read guardian enrollment with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.GuardianEnrollmentShow(enrollment)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func deleteGuardianEnrollmentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete a multi-factor authentication enrollment",
		Long: "Delete an MFA enrollment, allowing the user to re-enroll.\n\n" +
			"To delete interactively, use `auth0 guardian enrollments delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the enrollment id and the `--force` flag to skip confirmation.",
		Example: `  auth0 guardian enrollments delete
  auth0 guardian enrollments rm
  auth0 guardian enrollments delete <enrollment-id>
  auth0 guardian enrollments delete <enrollment-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var id string
			if len(args) == 0 {
				if err := guardianEnrollmentID.Ask(cmd, &id); err != nil {
					return err
				}
			} else {
				id = args[0]
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.Spinner("Deleting guardian enrollment", func() error {
				if err := cli.apiv3.GuardianEnrollment.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete guardian enrollment with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}
