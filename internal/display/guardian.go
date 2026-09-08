package display

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// guardianFactorView renders a single MFA factor and its enabled status.
type guardianFactorView struct {
	Name    string
	Enabled string

	raw interface{}
}

func (v *guardianFactorView) AsTableHeader() []string {
	return []string{"Factor", "Status"}
}

func (v *guardianFactorView) AsTableRow() []string {
	return []string{v.Name, v.Enabled}
}

func (v *guardianFactorView) KeyValues() [][]string {
	return [][]string{
		{"FACTOR", v.Name},
		{"STATUS", v.Enabled},
	}
}

func (v *guardianFactorView) Object() interface{} {
	return v.raw
}

func (r *Renderer) GuardianFactorList(factors []*managementv3.GuardianFactor) {
	resource := "guardian factors"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(factors)))

	if len(factors) == 0 {
		r.EmptyState(resource, "No MFA factors found")
		return
	}

	var results []View
	for _, factor := range factors {
		results = append(results, &guardianFactorView{
			Name:    string(factor.GetName()),
			Enabled: enabledStatus(factor.GetEnabled()),
			raw:     factor,
		})
	}

	r.Results(results)
}

func (r *Renderer) GuardianFactorSet(factor *managementv3.SetGuardianFactorResponseContent, name string) {
	r.Heading("guardian factor updated")
	r.Result(&guardianFactorView{
		Name:    name,
		Enabled: enabledStatus(factor.GetEnabled()),
		raw:     factor,
	})
}

// guardianPolicyView renders the tenant MFA policy list.
type guardianPolicyView struct {
	Policy string

	raw interface{}
}

func (v *guardianPolicyView) AsTableHeader() []string { return []string{"Policy"} }
func (v *guardianPolicyView) AsTableRow() []string    { return []string{v.Policy} }
func (v *guardianPolicyView) KeyValues() [][]string   { return [][]string{{"POLICY", v.Policy}} }
func (v *guardianPolicyView) Object() interface{}     { return v.raw }

func (r *Renderer) GuardianPolicyList(policies []managementv3.MfaPolicyEnum) {
	resource := "guardian policies"

	r.Heading(fmt.Sprintf("%s (%d)", resource, len(policies)))

	if len(policies) == 0 {
		r.EmptyState(resource, "No MFA policies configured")
		return
	}

	var results []View
	for _, policy := range policies {
		results = append(results, &guardianPolicyView{Policy: string(policy), raw: policy})
	}

	r.Results(results)
}

// guardianEnrollmentView renders a single MFA enrollment.
type guardianEnrollmentView struct {
	ID         string
	Status     string
	Name       string
	Identifier string
	Phone      string
	EnrolledAt string
	LastAuth   string

	raw interface{}
}

func (v *guardianEnrollmentView) AsTableHeader() []string {
	return []string{"ID", "Status", "Name", "Identifier"}
}

func (v *guardianEnrollmentView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Status, v.Name, v.Identifier}
}

func (v *guardianEnrollmentView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"STATUS", v.Status},
		{"NAME", v.Name},
		{"IDENTIFIER", v.Identifier},
		{"PHONE NUMBER", v.Phone},
		{"ENROLLED AT", v.EnrolledAt},
		{"LAST AUTH", v.LastAuth},
	}
}

func (v *guardianEnrollmentView) Object() interface{} { return v.raw }

func (r *Renderer) GuardianEnrollmentShow(enrollment *managementv3.GetGuardianEnrollmentResponseContent) {
	r.Heading("guardian enrollment")
	r.Result(&guardianEnrollmentView{
		ID:         enrollment.GetID(),
		Status:     string(enrollment.GetStatus()),
		Name:       orDash(enrollment.GetName()),
		Identifier: orDash(enrollment.GetIdentifier()),
		Phone:      orDash(enrollment.GetPhoneNumber()),
		EnrolledAt: orDash(enrollment.GetEnrolledAt()),
		LastAuth:   orDash(enrollment.GetLastAuth()),
		raw:        enrollment,
	})
}

// guardianEnrollmentTicketView renders a created enrollment ticket. The ticket
// URL is the actionable artifact the user shares with the enrollee.
type guardianEnrollmentTicketView struct {
	TicketID  string
	TicketURL string

	raw interface{}
}

func (v *guardianEnrollmentTicketView) AsTableHeader() []string {
	return []string{"Ticket ID", "Ticket URL"}
}

func (v *guardianEnrollmentTicketView) AsTableRow() []string {
	return []string{ansi.Faint(v.TicketID), v.TicketURL}
}

func (v *guardianEnrollmentTicketView) KeyValues() [][]string {
	return [][]string{
		{"TICKET ID", ansi.Faint(v.TicketID)},
		{"TICKET URL", v.TicketURL},
	}
}

func (v *guardianEnrollmentTicketView) Object() interface{} { return v.raw }

func (r *Renderer) GuardianEnrollmentTicketCreate(ticket *managementv3.CreateGuardianEnrollmentTicketResponseContent) {
	r.Heading("guardian enrollment ticket created")
	r.Result(&guardianEnrollmentTicketView{
		TicketID:  ticket.GetTicketID(),
		TicketURL: ticket.GetTicketURL(),
		raw:       ticket,
	})
}

// guardianDetailView renders an arbitrary key/value detail for the Guardian
// factor provider configuration commands. Callers compose the rows (masking
// secrets with MaskSecret) and pass the raw SDK response for --json output.
type guardianDetailView struct {
	rows [][]string
	raw  interface{}
}

func (v *guardianDetailView) AsTableHeader() []string { return []string{} }
func (v *guardianDetailView) AsTableRow() []string {
	row := make([]string, 0, len(v.rows))
	for _, kv := range v.rows {
		row = append(row, kv[1])
	}
	return row
}
func (v *guardianDetailView) KeyValues() [][]string { return v.rows }
func (v *guardianDetailView) Object() interface{}   { return v.raw }

// GuardianDetail renders a titled key/value detail for a Guardian factor
// provider configuration response.
func (r *Renderer) GuardianDetail(heading string, rows [][]string, raw interface{}) {
	r.Heading(heading)
	r.Result(&guardianDetailView{rows: rows, raw: raw})
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func enabledStatus(enabled bool) string {
	if enabled {
		return ansi.Green("enabled")
	}
	return ansi.Faint("disabled")
}

// MaskSecret masks a stored secret for display. It never reveals the value,
// only whether one is set. This keeps credentials out of terminal output and
// logs, per the CLI's secret-handling rules.
func MaskSecret(value string) string {
	if value == "" {
		return ansi.Faint("(not set)")
	}
	return "•••••••• " + ansi.Faint("(set)")
}
