package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestGuardianLegacyPhoneHint(t *testing.T) {
	t.Run("returns nil unchanged", func(t *testing.T) {
		assert.NoError(t, guardianLegacyPhoneHint(nil))
	})

	t.Run("passes through an unrelated error untouched", func(t *testing.T) {
		original := errors.New("failed to read phone provider: 404 not found")

		got := guardianLegacyPhoneHint(original)

		assert.Equal(t, original, got)
	})

	t.Run("augments the legacy phone-provider error with guidance", func(t *testing.T) {
		original := fmt.Errorf("failed to read phone provider: 403 %s", legacyPhoneProviderErrorCode)

		got := guardianLegacyPhoneHint(original)

		// The original error is preserved (wrapped) so callers keep the code.
		assert.ErrorIs(t, got, original)
		assert.Contains(t, got.Error(), legacyPhoneProviderErrorCode)

		// The hint names the actual mechanism and the recommended path forward.
		assert.Contains(t, got.Error(), "legacy_mfa_phone_provider migration flag")
		assert.Contains(t, got.Error(), "PATCH /api/v2/migrations")
		assert.Contains(t, got.Error(), "unified phone experience")
	})
}

func TestIsEmptyResponseErr(t *testing.T) {
	t.Run("false for nil", func(t *testing.T) {
		assert.False(t, isEmptyResponseErr(nil))
	})

	t.Run("false for an unrelated error", func(t *testing.T) {
		assert.False(t, isEmptyResponseErr(errors.New("403 forbidden")))
	})

	t.Run("true for the go-auth0 empty-body error", func(t *testing.T) {
		// Matches the SDK caller's wording for a response with no body.
		err := fmt.Errorf("expected a *management.GetGuardianFactorPhoneTemplatesResponseContent response, but the server responded with nothing")
		assert.True(t, isEmptyResponseErr(err))
	})
}

func TestSetGuardianPoliciesCmd(t *testing.T) {
	t.Run("requires --policy or --none when it cannot prompt", func(t *testing.T) {
		// In tests canPrompt is false (no TTY), so an empty invocation must not
		// silently clear every MFA policy.
		policy := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianPolicy: policy},
			renderer: testRenderer(),
		}

		cmd := setGuardianPoliciesCmd(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(
			t,
			cmd.Execute(),
			"--policy or --none is required when running non-interactively; supported values: all-applications, confidence-score, none",
		)
	})

	t.Run("--none clears every policy with an empty body", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		policy := mock.NewMockGuardianPolicyAPIV3(ctrl)
		policy.EXPECT().
			Set(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req managementv3.SetGuardianPoliciesRequestContent, _ ...option.RequestOption) (managementv3.SetGuardianPoliciesResponseContent, error) {
				assert.Empty(t, req)
				return managementv3.SetGuardianPoliciesResponseContent{}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianPolicy: policy},
			renderer: testRenderer(),
		}

		cmd := setGuardianPoliciesCmd(cli)
		cmd.SetArgs([]string{"--none"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("--policy sends the single selected policy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		policy := mock.NewMockGuardianPolicyAPIV3(ctrl)
		policy.EXPECT().
			Set(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req managementv3.SetGuardianPoliciesRequestContent, _ ...option.RequestOption) (managementv3.SetGuardianPoliciesResponseContent, error) {
				assert.Equal(t, managementv3.SetGuardianPoliciesRequestContent{managementv3.MfaPolicyEnumAllApplications}, req)
				return managementv3.SetGuardianPoliciesResponseContent{}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianPolicy: policy},
			renderer: testRenderer(),
		}

		cmd := setGuardianPoliciesCmd(cli)
		cmd.SetArgs([]string{"--policy", "all-applications"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("wraps the API error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		policy := mock.NewMockGuardianPolicyAPIV3(ctrl)
		policy.EXPECT().
			Set(gomock.Any(), gomock.Any()).
			Return(managementv3.SetGuardianPoliciesResponseContent{}, errors.New("boom"))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianPolicy: policy},
			renderer: testRenderer(),
		}

		cmd := setGuardianPoliciesCmd(cli)
		cmd.SetArgs([]string{"--none"})

		assert.EqualError(t, cmd.Execute(), "failed to set guardian policies: boom")
	})
}

func TestSetGuardianFactorCmd(t *testing.T) {
	t.Run("requires --enabled when it cannot prompt", func(t *testing.T) {
		// The factor is supplied positionally but --enabled is omitted; without a
		// TTY this must not silently disable the factor.
		factor := mock.NewMockGuardianFactorAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactor: factor},
			renderer: testRenderer(),
		}

		cmd := setGuardianFactorCmd(cli)
		cmd.SetArgs([]string{"sms"})

		assert.EqualError(
			t,
			cmd.Execute(),
			"--enabled is required when running non-interactively (use --enabled or --enabled=false)",
		)
	})

	t.Run("enables the named factor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factor := mock.NewMockGuardianFactorAPIV3(ctrl)
		factor.EXPECT().
			Set(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, name *managementv3.GuardianFactorNameEnum, req *managementv3.SetGuardianFactorRequestContent, _ ...option.RequestOption) (*managementv3.SetGuardianFactorResponseContent, error) {
				assert.Equal(t, managementv3.GuardianFactorNameEnumSms, *name)
				assert.True(t, req.Enabled)
				return &managementv3.SetGuardianFactorResponseContent{}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactor: factor},
			renderer: testRenderer(),
		}

		cmd := setGuardianFactorCmd(cli)
		cmd.SetArgs([]string{"sms", "--enabled"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("disables the named factor with --enabled=false", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		factor := mock.NewMockGuardianFactorAPIV3(ctrl)
		factor.EXPECT().
			Set(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, name *managementv3.GuardianFactorNameEnum, req *managementv3.SetGuardianFactorRequestContent, _ ...option.RequestOption) (*managementv3.SetGuardianFactorResponseContent, error) {
				assert.Equal(t, managementv3.GuardianFactorNameEnumEmail, *name)
				assert.False(t, req.Enabled)
				return &managementv3.SetGuardianFactorResponseContent{}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactor: factor},
			renderer: testRenderer(),
		}

		cmd := setGuardianFactorCmd(cli)
		cmd.SetArgs([]string{"email", "--enabled=false"})

		assert.NoError(t, cmd.Execute())
	})

	t.Run("rejects an unknown factor before calling the API", func(t *testing.T) {
		factor := mock.NewMockGuardianFactorAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactor: factor},
			renderer: testRenderer(),
		}

		cmd := setGuardianFactorCmd(cli)
		cmd.SetArgs([]string{"not-a-factor", "--enabled"})

		assert.ErrorContains(t, cmd.Execute(), `invalid factor "not-a-factor"`)
	})
}

func TestCreateGuardianEnrollmentTicketCmd(t *testing.T) {
	t.Run("requires --user-id when it cannot prompt", func(t *testing.T) {
		// --user-id is no longer a cobra-required flag (that made the interactive
		// prompt dead code), so the non-interactive guard must reject an empty
		// invocation before hitting the API.
		enrollment := mock.NewMockGuardianEnrollmentAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianEnrollment: enrollment},
			renderer: testRenderer(),
		}

		cmd := createGuardianEnrollmentTicketCmd(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), "--user-id is required when running non-interactively")
	})

	t.Run("creates the ticket with the supplied user id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		enrollment := mock.NewMockGuardianEnrollmentAPIV3(ctrl)
		enrollment.EXPECT().
			CreateTicket(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateGuardianEnrollmentTicketRequestContent, _ ...option.RequestOption) (*managementv3.CreateGuardianEnrollmentTicketResponseContent, error) {
				assert.Equal(t, "auth0|123", req.UserID)
				return &managementv3.CreateGuardianEnrollmentTicketResponseContent{}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianEnrollment: enrollment},
			renderer: testRenderer(),
		}

		cmd := createGuardianEnrollmentTicketCmd(cli)
		cmd.SetArgs([]string{"--user-id", "auth0|123"})

		assert.NoError(t, cmd.Execute())
	})
}

func TestSetGuardianPushEmptyInvocationGuards(t *testing.T) {
	// Each of these full-replace commands must reject an empty non-interactive
	// invocation instead of silently wiping the stored configuration.
	tests := []struct {
		name    string
		newCmd  func(*cli) *cobra.Command
		wantErr string
	}{
		{
			name:    "set-apns",
			newCmd:  setGuardianPushApnsCmd,
			wantErr: "set replaces the entire APNs configuration",
		},
		{
			name:    "set-sns",
			newCmd:  setGuardianPushSnsCmd,
			wantErr: "set replaces the entire SNS configuration",
		},
		{
			name:    "set-fcm",
			newCmd:  setGuardianPushFcmCmd,
			wantErr: "set replaces the entire FCM configuration, so --server-key is required",
		},
		{
			name:    "set-fcmv1",
			newCmd:  setGuardianPushFcmv1Cmd,
			wantErr: "set replaces the entire FCM v1 configuration, so --server-credentials is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A bare mock with no expectations fails the test if the API is called.
			push := mock.NewMockGuardianFactorPushAPIV3(gomock.NewController(t))

			cli := &cli{
				apiv3:    &auth0.APIV3{GuardianFactorPush: push},
				renderer: testRenderer(),
			}

			cmd := tt.newCmd(cli)
			cmd.SetArgs([]string{})

			assert.ErrorContains(t, cmd.Execute(), tt.wantErr)
		})
	}
}

func TestSetGuardianTwilioEmptyInvocationGuards(t *testing.T) {
	t.Run("phone set-twilio", func(t *testing.T) {
		phone := mock.NewMockGuardianFactorPhoneAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorPhone: phone},
			renderer: testRenderer(),
		}

		cmd := setGuardianPhoneTwilioCmd(cli)
		cmd.SetArgs([]string{})

		assert.ErrorContains(t, cmd.Execute(), "set replaces the entire Twilio configuration")
	})

	t.Run("sms set-twilio", func(t *testing.T) {
		sms := mock.NewMockGuardianFactorSmsAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorSms: sms},
			renderer: testRenderer(),
		}

		cmd := setGuardianSmsTwilioCmd(cli)
		cmd.SetArgs([]string{})

		assert.ErrorContains(t, cmd.Execute(), "set replaces the entire Twilio configuration")
	})
}

func TestSetGuardianPhoneMessageTypesCmd(t *testing.T) {
	t.Run("requires --message-type when it cannot prompt", func(t *testing.T) {
		phone := mock.NewMockGuardianFactorPhoneAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorPhone: phone},
			renderer: testRenderer(),
		}

		cmd := setGuardianPhoneMessageTypesCmd(cli)
		cmd.SetArgs([]string{})

		assert.ErrorContains(t, cmd.Execute(), "--message-type is required when running non-interactively")
	})
}

func TestSetGuardianTemplatesGuards(t *testing.T) {
	t.Run("phone set-templates requires a message flag when it cannot prompt", func(t *testing.T) {
		phone := mock.NewMockGuardianFactorPhoneAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorPhone: phone},
			renderer: testRenderer(),
		}

		cmd := setGuardianPhoneTemplatesCmd(cli)
		cmd.SetArgs([]string{})

		assert.ErrorContains(t, cmd.Execute(), "set replaces the phone templates")
	})

	t.Run("sms set-templates requires a message flag when it cannot prompt", func(t *testing.T) {
		sms := mock.NewMockGuardianFactorSmsAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorSms: sms},
			renderer: testRenderer(),
		}

		cmd := setGuardianSmsTemplatesCmd(cli)
		cmd.SetArgs([]string{})

		assert.ErrorContains(t, cmd.Execute(), "set replaces the SMS templates")
	})
}

func TestSetGuardianProviderRequiresValue(t *testing.T) {
	t.Run("phone set-provider requires --provider when it cannot prompt", func(t *testing.T) {
		phone := mock.NewMockGuardianFactorPhoneAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorPhone: phone},
			renderer: testRenderer(),
		}

		cmd := setGuardianPhoneProviderCmd(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), "--provider is required: valid values are auth0, twilio, phone-message-hook")
	})

	t.Run("sms set-provider requires --provider when it cannot prompt", func(t *testing.T) {
		sms := mock.NewMockGuardianFactorSmsAPIV3(gomock.NewController(t))

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorSms: sms},
			renderer: testRenderer(),
		}

		cmd := setGuardianSmsProviderCmd(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), "--provider is required: valid values are auth0, twilio, phone-message-hook")
	})
}

func TestUpdateGuardianDuoSettingsCmd(t *testing.T) {
	t.Run("re-fetches the full settings after the PATCH", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		duo := mock.NewMockGuardianFactorDuoAPIV3(ctrl)

		// The PATCH response only echoes the sent fields, so the command must
		// Get -> Update -> Get to render the complete current state.
		gomock.InOrder(
			duo.EXPECT().
				Get(gomock.Any()).
				Return(&managementv3.GetGuardianFactorDuoSettingsResponseContent{
					Host: auth0.String("api-old.duosecurity.com"),
					Ikey: auth0.String("ikey-1"),
				}, nil),
			duo.EXPECT().
				Update(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, req *managementv3.UpdateGuardianFactorDuoSettingsRequestContent, _ ...option.RequestOption) (*managementv3.UpdateGuardianFactorDuoSettingsResponseContent, error) {
					// Only the host was provided, so only it is sent.
					assert.Equal(t, "api-new.duosecurity.com", *req.Host)
					assert.Nil(t, req.Ikey)
					assert.Nil(t, req.Skey)
					return &managementv3.UpdateGuardianFactorDuoSettingsResponseContent{}, nil
				}),
			duo.EXPECT().
				Get(gomock.Any()).
				Return(&managementv3.GetGuardianFactorDuoSettingsResponseContent{
					Host: auth0.String("api-new.duosecurity.com"),
					Ikey: auth0.String("ikey-1"),
				}, nil),
		)

		cli := &cli{
			apiv3:    &auth0.APIV3{GuardianFactorDuo: duo},
			renderer: testRenderer(),
		}

		cmd := updateGuardianDuoSettingsCmd(cli)
		cmd.SetArgs([]string{"--host", "api-new.duosecurity.com"})

		assert.NoError(t, cmd.Execute())
	})
}
