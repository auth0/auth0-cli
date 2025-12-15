package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type tokenExchangeProfileView struct {
	ID               string
	Name             string
	SubjectTokenType string
	ActionID         string
	Type             string
	CreatedAt        string
	UpdatedAt        string
	raw              interface{}
}

func (v *tokenExchangeProfileView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Subject Token Type", "Action ID"}
}

func (v *tokenExchangeProfileView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Name,
		v.Type,
		v.SubjectTokenType,
		ansi.Faint(v.ActionID),
	}
}

func (v *tokenExchangeProfileView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"SUBJECT TOKEN TYPE", v.SubjectTokenType},
		{"ACTION ID", ansi.Faint(v.ActionID)},
		{"CREATED AT", v.CreatedAt},
		{"UPDATED AT", v.UpdatedAt},
	}
}

func (v *tokenExchangeProfileView) Object() interface{} {
	return v.raw
}

func (r *Renderer) TokenExchangeProfileList(profiles []*management.TokenExchangeProfile) {
	resource := "token exchange profiles"

	r.Heading(resource)

	if len(profiles) == 0 {
		r.EmptyState(resource, "Use 'auth0 token-exchange create' to add one")
		return
	}

	var res []View
	for _, p := range profiles {
		res = append(res, makeTokenExchangeProfileView(p))
	}

	r.Results(res)
}

func (r *Renderer) TokenExchangeProfileShow(profile *management.TokenExchangeProfile) {
	r.Heading("token exchange profile")
	r.Result(makeTokenExchangeProfileView(profile))
}

func (r *Renderer) TokenExchangeProfileCreate(profile *management.TokenExchangeProfile) {
	r.Heading("token exchange profile created")
	r.Result(makeTokenExchangeProfileView(profile))
}

func (r *Renderer) TokenExchangeProfileUpdate(profile *management.TokenExchangeProfile) {
	r.Heading("token exchange profile updated")
	r.Result(makeTokenExchangeProfileView(profile))
}

func makeTokenExchangeProfileView(profile *management.TokenExchangeProfile) *tokenExchangeProfileView {
	view := &tokenExchangeProfileView{
		ID:               profile.GetID(),
		Name:             profile.GetName(),
		SubjectTokenType: profile.GetSubjectTokenType(),
		ActionID:         profile.GetActionID(),
		Type:             profile.GetType(),
		raw:              profile,
	}

	if profile.CreatedAt != nil {
		view.CreatedAt = timeAgo(profile.GetCreatedAt())
	}

	if profile.UpdatedAt != nil {
		view.UpdatedAt = timeAgo(profile.GetUpdatedAt())
	}

	return view
}
