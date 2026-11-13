package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
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

// ActionUpdater is the part of the Actions API that apply uses.
type ActionUpdater interface {
	Create(ctx context.Context, a *management.Action, opts ...management.RequestOption) error
	Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.Action, error)
	Update(ctx context.Context, id string, a *management.Action, opts ...management.RequestOption) error
	Deploy(ctx context.Context, id string, opts ...management.RequestOption) (*management.ActionVersion, error)
	Bindings(ctx context.Context, triggerID string, opts ...management.RequestOption) (*management.ActionBindingList, error)
	UpdateBindings(ctx context.Context, triggerID string, b []*management.ActionBinding, opts ...management.RequestOption) error
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
	// Actions is only used for a change of the post-login Action.
	Actions ActionUpdater
	// BuildPoll is how long apply waits between two looks at an Action that is
	// being built. It defaults to a second.
	BuildPoll time.Duration
}

// Apply makes the changes of the plan in plan order, one Management API call
// each, then writes the config file. It stops at the first failure. A failure
// after the first change returns a *PartialError; a failure of the first change
// returns the error itself, as no other change was made. A failure of the Action
// change says that the Action may exist, because its steps (create, deploy, bind)
// can have been done before one failed. On success it returns the description of
// each change made.
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
				return nil, firstFailure(change, err)
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

// firstFailure is the error of a failed first change. The change of the Action is
// several calls, so when it fails the Action may exist already; running setup
// again finds it by name, then deploys and binds it. An *InputError already says
// what to do and is returned as it is.
func firstFailure(change Change, err error) error {
	var input *InputError
	if change.Kind != KindAction || errors.As(err, &input) {
		return err
	}

	return &actionStoppedError{err: err}
}

// actionStoppedError is a failure of the change of the Action that says the Action
// may exist already.
type actionStoppedError struct{ err error }

func (e *actionStoppedError) Error() string {
	return e.err.Error() + "\n  The Action may exist already. Running the command again finds it and continues from here."
}

func (e *actionStoppedError) Unwrap() error { return e.err }

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
	case KindAction:
		if err := a.applyAction(ctx, change, state.Action); err != nil {
			return err
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
			request.PersonalAgentProtocolCompanyDomains = companyDomainsWith(state.Tenant.CompanyDomains, in.CompanyDomain)
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

const (
	// DefaultBuildPoll and maxBuildChecks bound the wait for a draft to be built:
	// a minute at the default.
	defaultBuildPoll = time.Second
	maxBuildChecks   = 60
)

// applyAction brings the post-login Action to what the change sets: it creates
// the Action or updates its code, deploys it, and binds it as the last Action of
// the post-login trigger. Each step is done only when the change has the field
// for it, so that running setup again continues from where a failure stopped.
func (a *Applier) applyAction(ctx context.Context, change Change, state *ActionState) error {
	id := state.ID

	for _, field := range change.Fields {
		switch field.Name {
		case fieldAction:
			var err error
			if id, err = a.deployAction(ctx, state); err != nil {
				return err
			}
		case fieldActionOrder:
			if err := a.checkFlow(ctx, state); err != nil {
				return err
			}

			if err := a.Actions.UpdateBindings(ctx, postLoginTrigger, flowBindings(state, id)); err != nil {
				return fmt.Errorf("failed to bind the Action to the post-login trigger: %w", err)
			}
		}
	}

	return nil
}

// checkFlow stops the update of the bindings when the flow is not the one that
// was read. The update replaces the whole list, so a binding that someone added
// since the read would be lost. The bindings are read again just before the
// update and compared with the ones the plan was made from: the same bindings in
// the same order, by binding ID and Action.
func (a *Applier) checkFlow(ctx context.Context, state *ActionState) error {
	bindings, err := readBindings(ctx, a.Actions)
	if err != nil {
		return err
	}

	// A binding that setup could not write back is a change of the flow too.
	current, err := flowOf(bindings)
	if err != nil || !sameFlow(state.Flow, current) {
		return &InputError{
			Reason:  ReasonActionFlowChanged,
			Message: "The post-login flow changed while setup was running, so setup left it as it is.",
			Fix:     "Run the command again. The Action is deployed by then, and setup binds it to the flow as it is.",
		}
	}

	return nil
}

// sameFlow tells whether two flows have the same bindings in the same order. It
// compares what the update writes back, and the binding IDs, which the API
// changes whenever the list is replaced.
func sameFlow(a, b []FlowAction) bool {
	return slices.EqualFunc(a, b, func(x, y FlowAction) bool {
		return x.BindingID == y.BindingID && x.RefType == y.RefType && x.RefValue == y.RefValue && x.DisplayName == y.DisplayName
	})
}

// deployAction creates the Action, or updates its code when it differs, and
// deploys it once it is built. It returns the ID of the Action.
func (a *Applier) deployAction(ctx context.Context, state *ActionState) (string, error) {
	id := state.ID

	// An update sends the name and the triggers too, because the API takes the
	// request as the whole Action.
	action := &management.Action{
		Name:              auth0.String(OfflineAccessActionName),
		SupportedTriggers: []management.ActionTrigger{{ID: auth0.String(postLoginTrigger), Version: auth0.String(postLoginTriggerVersion)}},
		Code:              auth0.String(OfflineAccessActionSource),
	}

	switch {
	case id == "":
		if err := a.Actions.Create(ctx, action); err != nil {
			return "", fmt.Errorf("failed to create the Action: %w", err)
		}

		id = action.GetID()
	case !state.CodeCurrent:
		if err := a.Actions.Update(ctx, id, action); err != nil {
			return "", fmt.Errorf("failed to update the Action: %w", err)
		}
	}

	if err := a.waitForBuild(ctx, id); err != nil {
		return "", err
	}

	if _, err := a.Actions.Deploy(ctx, id); err != nil {
		return "", fmt.Errorf("failed to deploy the Action: %w", err)
	}

	return id, nil
}

// waitForBuild waits until the draft of the Action is built, which a deploy needs.
func (a *Applier) waitForBuild(ctx context.Context, id string) error {
	poll := a.BuildPoll
	if poll <= 0 {
		poll = defaultBuildPoll
	}

	for check := 0; check < maxBuildChecks; check++ {
		action, err := a.Actions.Read(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to read the Action: %w", err)
		}

		switch action.GetStatus() {
		case management.ActionStatusBuilt:
			return nil
		case management.ActionStatusFailed:
			return errors.New("the Action failed to build")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}

	return errors.New("the Action was not built in time")
}

// flowBindings builds the list that replaces the bindings of the post-login
// trigger: every Action that is bound now, in its order, then the Action of setup.
// Each binding keeps its display name.
func flowBindings(state *ActionState, id string) []*management.ActionBinding {
	others := state.others()
	bindings := make([]*management.ActionBinding, 0, len(others)+1)

	for _, other := range others {
		bindings = append(bindings, bindingOf(other.RefType, other.RefValue, other.DisplayName))
	}

	return append(bindings, bindingOf(management.ActionBindingReferenceByID, id, OfflineAccessActionName))
}

func bindingOf(refType, refValue, displayName string) *management.ActionBinding {
	binding := &management.ActionBinding{Ref: &management.ActionBindingReference{Type: auth0.String(refType), Value: auth0.String(refValue)}}
	if displayName != "" {
		binding.DisplayName = auth0.String(displayName)
	}

	return binding
}
