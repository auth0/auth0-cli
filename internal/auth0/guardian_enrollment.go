//go:generate mockgen -source=guardian_enrollment.go -destination=mock/guardian_enrollment_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// GuardianEnrollmentAPIV3 is the V3 SDK interface for multi-factor
// authentication (MFA) enrollments (/guardian/enrollments).
type GuardianEnrollmentAPIV3 interface {
	// CreateTicket creates an MFA enrollment ticket for a user and, optionally,
	// emails it to them.
	//
	// Required scope: `create:guardian_enrollment_tickets`.
	CreateTicket(ctx context.Context, request *managementv3.CreateGuardianEnrollmentTicketRequestContent, opts ...option.RequestOption) (*managementv3.CreateGuardianEnrollmentTicketResponseContent, error)

	// Get retrieves details for a single MFA enrollment by ID.
	//
	// Required scope: `read:guardian_enrollments`.
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.GetGuardianEnrollmentResponseContent, error)

	// Delete removes a single MFA enrollment, allowing the user to re-enroll.
	//
	// Required scope: `delete:guardian_enrollments`.
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error
}
