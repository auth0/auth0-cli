package cli

import (
	"context"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
)

// staticPickerOptions returns a pickerOptionsFunc over a fixed set of string
// values, used for interactive multi-select of enum values.
func staticPickerOptions(values []string) pickerOptionsFunc {
	return func(_ context.Context) (pickerOptions, error) {
		var opts pickerOptions
		for _, v := range values {
			opts = append(opts, pickerOption{value: v, label: v})
		}
		return opts, nil
	}
}

// messageTypesForDisplay renders a list of phone message types for output.
func messageTypesForDisplay(types []managementv3.GuardianFactorPhoneFactorMessageTypeEnum) string {
	if len(types) == 0 {
		return "-"
	}
	values := make([]string, 0, len(types))
	for _, t := range types {
		values = append(values, string(t))
	}
	return strings.Join(values, ", ")
}

// Provider selection option lists.
var (
	guardianSmsProviderOptions = []string{
		string(managementv3.GuardianFactorsProviderSmsProviderEnumAuth0),
		string(managementv3.GuardianFactorsProviderSmsProviderEnumTwilio),
		string(managementv3.GuardianFactorsProviderSmsProviderEnumPhoneMessageHook),
	}
	guardianPushProviderOptions = []string{
		string(managementv3.GuardianFactorsProviderPushNotificationProviderDataEnumGuardian),
		string(managementv3.GuardianFactorsProviderPushNotificationProviderDataEnumSns),
		string(managementv3.GuardianFactorsProviderPushNotificationProviderDataEnumDirect),
	}
	guardianMessageTypeOptions = []string{
		string(managementv3.GuardianFactorPhoneFactorMessageTypeEnumSms),
		string(managementv3.GuardianFactorPhoneFactorMessageTypeEnumVoice),
	}
)

// Shared flags reused across the phone, sms, push and duo provider commands.
// Each command registers only the flags it needs, so sharing the definitions is
// safe and keeps help text consistent.
var (
	guardianProvider = Flag{
		Name:      "Provider",
		LongForm:  "provider",
		ShortForm: "p",
		Help:      "Provider to use for the factor.",
	}
	guardianMessageType = Flag{
		Name:     "Message Type",
		LongForm: "message-type",
		Help:     "Message type to enable. Repeat the flag for multiple types. Supported values: sms, voice.",
	}
	guardianEnrollmentMessage = Flag{
		Name:     "Enrollment Message",
		LongForm: "enrollment-message",
		Help:     "Message sent to the user when they enroll.",
	}
	guardianVerificationMessage = Flag{
		Name:     "Verification Message",
		LongForm: "verification-message",
		Help:     "Message sent to the user when they verify.",
	}

	// Twilio.
	guardianTwilioFrom = Flag{
		Name:     "From",
		LongForm: "from",
		Help:     "Twilio 'from' phone number.",
	}
	guardianTwilioMessagingServiceSid = Flag{
		Name:     "Messaging Service SID",
		LongForm: "messaging-service-sid",
		Help:     "Twilio messaging service SID.",
	}
	guardianTwilioSid = Flag{
		Name:     "SID",
		LongForm: "sid",
		Help:     "Twilio account SID.",
	}
	guardianTwilioAuthToken = Flag{
		Name:     "Auth Token",
		LongForm: "auth-token",
		Help:     "Twilio authentication token.",
	}

	// APNs.
	guardianApnsBundleID = Flag{
		Name:         "Bundle ID",
		LongForm:     "bundle-id",
		Help:         "Apple app bundle identifier.",
		AlwaysPrompt: true,
	}
	guardianApnsSandbox = Flag{
		Name:         "Sandbox",
		LongForm:     "sandbox",
		Help:         "Whether to use the APNs sandbox environment.",
		AlwaysPrompt: true,
	}
	guardianApnsP12 = Flag{
		Name:         "P12",
		LongForm:     "p12",
		Help:         "Base64-encoded .p12 certificate for APNs.",
		AlwaysPrompt: true,
	}

	// FCM.
	guardianFcmServerKey = Flag{
		Name:     "Server Key",
		LongForm: "server-key",
		Help:     "Google FCM (legacy) server key.",
	}
	guardianFcmServerCredentials = Flag{
		Name:     "Server Credentials",
		LongForm: "server-credentials",
		Help:     "Google FCM v1 service account credentials (JSON).",
	}

	// SNS.
	guardianSnsAccessKeyID = Flag{
		Name:         "AWS Access Key ID",
		LongForm:     "aws-access-key-id",
		Help:         "AWS access key ID for SNS.",
		AlwaysPrompt: true,
	}
	guardianSnsSecretAccessKey = Flag{
		Name:         "AWS Secret Access Key",
		LongForm:     "aws-secret-access-key",
		Help:         "AWS secret access key for SNS.",
		AlwaysPrompt: true,
	}
	guardianSnsRegion = Flag{
		Name:         "AWS Region",
		LongForm:     "aws-region",
		Help:         "AWS region for SNS.",
		AlwaysPrompt: true,
	}
	guardianSnsApnsArn = Flag{
		Name:         "APNs Platform Application ARN",
		LongForm:     "apns-platform-arn",
		Help:         "SNS APNs platform application ARN.",
		AlwaysPrompt: true,
	}
	guardianSnsGcmArn = Flag{
		Name:         "GCM Platform Application ARN",
		LongForm:     "gcm-platform-arn",
		Help:         "SNS GCM platform application ARN.",
		AlwaysPrompt: true,
	}

	// Duo.
	guardianDuoIkey = Flag{
		Name:         "Integration Key",
		LongForm:     "ikey",
		Help:         "Duo integration key.",
		AlwaysPrompt: true,
	}
	guardianDuoSkey = Flag{
		Name:         "Secret Key",
		LongForm:     "skey",
		Help:         "Duo secret key.",
		AlwaysPrompt: true,
	}
	guardianDuoHost = Flag{
		Name:         "API Hostname",
		LongForm:     "host",
		Help:         "Duo API hostname.",
		AlwaysPrompt: true,
	}
)
