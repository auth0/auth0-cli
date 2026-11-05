package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// TenantSettingsUpdater is the part of the tenant settings API that apply uses.
type TenantSettingsUpdater interface {
	Update(ctx context.Context, request *managementv3.UpdateTenantSettingsRequestContent, opts ...option.RequestOption) (*managementv3.UpdateTenantSettingsResponseContent, error)
}

// ResourceServerUpdater is the part of the resource servers API that apply uses.
type ResourceServerUpdater interface {
	Update(ctx context.Context, id string, request *managementv3.UpdateResourceServerRequestContent, opts ...option.RequestOption) (*managementv3.UpdateResourceServerResponseContent, error)
}

// ClientUpdater is the part of the clients API that apply uses.
type ClientUpdater interface {
	Update(ctx context.Context, id string, request *managementv3.UpdateClientRequestContent, opts ...option.RequestOption) (*managementv3.UpdateClientResponseContent, error)
}

// GrantCreator is the part of the client grants API that apply uses.
type GrantCreator interface {
	Create(ctx context.Context, request *managementv3.CreateClientGrantRequestContent, opts ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error)
}

// Outcome says how far an apply got.
type Outcome struct {
	// Done is the number of changes of the plan that were made, in plan order.
	Done int
	// Failure is why the change after the done ones failed. It is nil when the
	// apply made every change.
	Failure error
}

// Approval says what a plan needs before it is applied.
type Approval int

const (
	// Proceed means the plan can be applied now.
	Proceed Approval = iota
	// Ask means the user must confirm the plan.
	Ask
	// Refuse means the plan needs confirmation but there is nobody to ask.
	Refuse
)

// Decide applies the confirmation rules: --yes goes ahead, a terminal asks, and
// anything else is refused so that nothing changes unattended.
func Decide(yes, interactive bool) Approval {
	switch {
	case yes:
		return Proceed
	case interactive:
		return Ask
	default:
		return Refuse
	}
}

// PartialError reports an apply that stopped after changing some things.
type PartialError struct {
	// The changes that were made, the one that failed and those not tried, by
	// description.
	Applied   []string
	Failed    string
	Remaining []string
	Err       error
}

// Error lists what was done and what is left.
func (e *PartialError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "stopped partway: %v\n  applied: %s\n  failed: %s", e.Err, strings.Join(e.Applied, "; "), e.Failed)

	if len(e.Remaining) > 0 {
		fmt.Fprintf(&b, "\n  remaining: %s", strings.Join(e.Remaining, "; "))
	}

	b.WriteString("\n  Running the command again continues from here.")

	return b.String()
}

// Unwrap returns the error that stopped the apply.
func (e *PartialError) Unwrap() error { return e.Err }

// ErrorDetails gives the JSON error output the applied, failed and remaining
// changes.
func (e *PartialError) ErrorDetails() json.RawMessage {
	remaining := e.Remaining
	if remaining == nil {
		remaining = []string{}
	}

	details, _ := json.Marshal(map[string]any{"code": "partial", "applied": e.Applied, "failed": e.Failed, "remaining": remaining})

	return details
}

// Applier makes the changes of a plan through the Management API.
type Applier struct {
	Tenant  TenantSettingsUpdater
	Servers ResourceServerUpdater
	Clients ClientUpdater
	Grants  GrantCreator
}

// Apply makes the changes of the plan in plan order, one Management API call
// each, then writes the config file. It stops at the first failure. A failure
// after the first change returns a *PartialError; a failure of the first change
// returns the error itself, as nothing was changed. On success it returns the
// description of each change made.
func (a *Applier) Apply(ctx context.Context, state *State, in Inputs, plan Plan) ([]string, error) {
	scopes, err := resolveScopes(state, in)
	if err != nil {
		return nil, err
	}

	cfg, err := derive(state, in, scopes)
	if err != nil {
		return nil, err
	}

	applied := make([]string, 0, len(plan.Changes))

	for i, change := range plan.Changes {
		if err := a.apply(ctx, change, state, in, scopes, cfg); err != nil {
			if i == 0 {
				return nil, err
			}

			remaining := make([]string, 0, len(plan.Changes)-i-1)
			for _, next := range plan.Changes[i+1:] {
				remaining = append(remaining, next.Description)
			}

			return nil, &PartialError{Applied: applied, Failed: change.Description, Remaining: remaining, Err: err}
		}

		applied = append(applied, change.Description)
	}

	return applied, nil
}

func (a *Applier) apply(ctx context.Context, change Change, state *State, in Inputs, scopes map[string]apiScopes, cfg *config.Config) error {
	switch change.Kind {
	case KindTenant:
		if _, err := a.Tenant.Update(ctx, tenantRequest(change, state, in)); err != nil {
			return fmt.Errorf("failed to update the tenant settings: %w", err)
		}
	case KindAPI:
		if _, err := a.Servers.Update(ctx, apiID(state, change.target), serverRequest(change, apiOf(state, change.target), scopes[change.target])); err != nil {
			return fmt.Errorf("failed to update the API %s: %w", change.target, err)
		}
	case KindApp:
		if _, err := a.Clients.Update(ctx, state.App.ClientID, clientRequest(change)); err != nil {
			return fmt.Errorf("failed to update the app %s: %w", state.App.Name, err)
		}
	case KindGrant:
		if _, err := a.Grants.Create(ctx, grantRequest(state.App, change.target, guestScopeNames(in, change.target, scopes[change.target]))); err != nil {
			return fmt.Errorf("failed to create the guest grant of %s for %s: %w", state.App.Name, change.target, err)
		}
	case KindFile:
		if err := config.Write(in.Out, cfg); err != nil {
			return fmt.Errorf("failed to write %s: %w", in.Out, err)
		}
	}

	return nil
}

// tenantRequest builds the one request that sets every changed tenant setting.
// The company domain is added to the domains the tenant lists.
func tenantRequest(change Change, state *State, in Inputs) *managementv3.UpdateTenantSettingsRequestContent {
	request := &managementv3.UpdateTenantSettingsRequestContent{}

	for _, field := range change.Fields {
		switch field.Name {
		case fieldCIMD:
			request.ClientIDMetadataDocumentSupported = managementv3.Bool(true)
		case fieldIss:
			request.AuthorizationResponseIssParameterSupported = managementv3.Bool(true)
		case fieldResourceProfile:
			profile := managementv3.TenantSettingsResourceParameterProfileCompatibility
			request.ResourceParameterProfile = &profile
		case fieldDefaultAudience:
			request.DefaultAudience = managementv3.String(field.Value)
		case FieldCompanyDomains:
			request.PersonalAgentProtocolCompanyDomains = append(append([]string{}, state.Tenant.CompanyDomains...), companyDomainURL(in.CompanyDomain))
		}
	}

	return request
}

// serverRequest builds the request that sets the Poppy settings of an API. The
// scopes it sends are all the scopes of the API, so none is dropped, and the
// user and client policies are sent back as they are, because the update
// replaces the whole subject_type_authorization object.
func serverRequest(change Change, api API, scopes apiScopes) *managementv3.UpdateResourceServerRequestContent {
	request := &managementv3.UpdateResourceServerRequestContent{}

	for _, field := range change.Fields {
		switch field.Name {
		case fieldScopes:
			for _, scope := range scopes.All {
				request.Scopes = append(request.Scopes, &managementv3.ResourceServerScope{
					Value:       scope.Name,
					Description: managementv3.String(scope.Description),
				})
			}
		case fieldOfflineAccess:
			request.AllowOfflineAccess = managementv3.Bool(true)
		case fieldAgentClaims:
			claims := managementv3.ResourceServerAgentSubjectClaimsSchemaEnumAuth0V1
			request.AgentSubjectClaims = &claims
		case fieldPoppy:
			request.UseForPersonalAgentProtocol = managementv3.Bool(true)
		case fieldGuestPolicy:
			policy := managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUserPolicyEnumRequireClientGrant
			subjects := &managementv3.ResourceServerSubjectTypeAuthorization{
				AnonymousUser: &managementv3.ResourceServerSubjectTypeAuthorizationAnonymousUser{Policy: &policy},
			}
			if api.UserPolicy != "" {
				user := managementv3.ResourceServerSubjectTypeAuthorizationUserPolicyEnum(api.UserPolicy)
				subjects.User = &managementv3.ResourceServerSubjectTypeAuthorizationUser{Policy: &user}
			}
			if api.ClientPolicy != "" {
				client := managementv3.ResourceServerSubjectTypeAuthorizationClientPolicyEnum(api.ClientPolicy)
				subjects.Client = &managementv3.ResourceServerSubjectTypeAuthorizationClient{Policy: &client}
			}
			request.SubjectTypeAuthorization = subjects
		}
	}

	return request
}

// clientRequest builds the request that sets the browser join settings of the
// web app. It never touches the callbacks or the session transfer settings.
func clientRequest(change Change) *managementv3.UpdateClientRequestContent {
	request := &managementv3.UpdateClientRequestContent{}

	for _, field := range change.Fields {
		switch field.Name {
		case fieldLoginURI:
			request.InitiateLoginURI = managementv3.String(field.Value)
		case fieldBrowserJoin:
			request.AllowPersonalAgentBrowserJoin = managementv3.Bool(true)
		}
	}

	return request
}

func grantRequest(app App, audience string, names []string) *managementv3.CreateClientGrantRequestContent {
	subject := managementv3.ClientGrantSubjectTypeEnumAnonymousUser

	return &managementv3.CreateClientGrantRequestContent{
		ClientID:    &app.ClientID,
		Audience:    audience,
		Scope:       names,
		SubjectType: &subject,
	}
}

// apiOf returns the API with the given identifier.
func apiOf(state *State, identifier string) API {
	for _, api := range state.APIs {
		if api.Identifier == identifier {
			return api
		}
	}

	return API{}
}

// apiID returns the ID of the API with the given identifier.
func apiID(state *State, identifier string) string {
	for _, api := range state.APIs {
		if api.Identifier == identifier {
			return api.ID
		}
	}

	return ""
}
