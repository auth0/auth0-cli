package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// legacyPhoneProviderErrorCode is the errorCode the Management API returns when
// a tenant uses the deprecated Guardian phone/SMS provider, template or Twilio
// endpoints without the legacy phone-provider flags enabled.
const legacyPhoneProviderErrorCode = "legacy_mfa_phone_provider_not_allowed"

// guardianLegacyPhoneHint augments the deprecated legacy phone-provider error
// with actionable guidance. These endpoints are gated behind the tenant flags
// legacy_mfa_phone_provider and legacy_passwordless_phone_provider; without them
// the API refuses the request in favour of Tenant Phone Settings.
func guardianLegacyPhoneHint(err error) error {
	if err == nil || !strings.Contains(err.Error(), legacyPhoneProviderErrorCode) {
		return err
	}

	return fmt.Errorf(
		"%w\n\n"+
			"This is a deprecated Guardian phone/SMS provider endpoint, gated behind the tenant's "+
			"legacy_mfa_phone_provider migration flag. That flag is toggled via PATCH /api/v2/migrations, "+
			"which requires Auth0 Dashboard (session) access and is not available to Management API tokens, "+
			"so it cannot be enabled from the CLI. Use the unified phone experience instead: configure a "+
			"tenant phone provider (Dashboard: Authentication > Phone), or set the MFA phone provider to "+
			"'phone-message-hook' backed by a send-phone-message action",
		err,
	)
}

// emptyResponseErrorFragment identifies the go-auth0 SDK error returned when the
// API responds with an empty body (see the SDK's caller: "expected a %T
// response, but the server responded with nothing"). The Guardian phone/SMS
// template endpoints return an empty body when no templates are configured,
// which is a valid state rather than a failure.
const emptyResponseErrorFragment = "server responded with nothing"

// isEmptyResponseErr reports whether err is the go-auth0 empty-body error.
func isEmptyResponseErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), emptyResponseErrorFragment)
}

func guardianCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardian",
		Short: "Manage multi-factor authentication (Guardian)",
		Long: "Manage Auth0 multi-factor authentication (MFA), also known as Guardian. " +
			"Configure MFA policies, factors and their providers, and manage user enrollments.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(guardianPoliciesCmd(cli))
	cmd.AddCommand(guardianEnrollmentsCmd(cli))
	cmd.AddCommand(guardianFactorsCmd(cli))

	return cmd
}
