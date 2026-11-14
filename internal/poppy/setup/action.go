package setup

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/management"
)

// OfflineAccessActionName is the name of the post-login Action that setup creates
// to always issue refresh tokens. Setup finds the Action by this exact name.
const OfflineAccessActionName = "Poppy: always issue refresh tokens"

const (
	// PostLoginTrigger is the trigger the Action runs on.
	postLoginTrigger = management.ActionTriggerPostLogin
	// PostLoginTriggerVersion is the version of the trigger the Action supports.
	postLoginTriggerVersion = "v3"
)

// apisPlaceholder is where the template of the Action takes the list of APIs.
const apisPlaceholder = "__POPPY_APIS__"

//go:embed offline_access_action.js
var offlineAccessActionTemplate string

// OfflineAccessActionCode returns the code of the Action for the identifiers of
// the Poppy APIs it acts for. The list is sorted and has no duplicates, so the
// same APIs always give the same code.
func OfflineAccessActionCode(identifiers []string) string {
	list := slices.Clone(identifiers)
	slices.Sort(list)
	list = slices.Compact(list)

	// A list of strings always marshals, and JSON is a valid JavaScript array.
	encoded, _ := json.Marshal(list)
	if list == nil {
		encoded = []byte("[]")
	}

	return strings.Replace(offlineAccessActionTemplate, apisPlaceholder, string(encoded), 1)
}

// ActionState is the post-login Action that always issues refresh tokens, and the
// flow it runs in.
type ActionState struct {
	// ID is the ID of the Action, empty when the tenant has no Action of that name.
	ID string
	// Code is the code the Action should have: the template with the Poppy APIs.
	Code string
	// CodeCurrent tells whether the code of the Action is that code.
	CodeCurrent bool
	// Deployed tells whether that code is the deployed version.
	Deployed bool
	// Flow lists the Actions bound to the post-login trigger, in the order they run.
	Flow []FlowAction
}

// FlowAction is an Action bound to the post-login trigger.
type FlowAction struct {
	// BindingID is the ID of the binding. The API gives every binding a new ID
	// when the list is replaced, so a change of the list by someone else shows as
	// a different ID.
	BindingID string
	// RefType and RefValue refer to the Action the way the bindings update does.
	RefType  string
	RefValue string
	// DisplayName is the display name of the binding, as the tenant has it, which
	// the update sends back.
	DisplayName string
	// Name is what the plan shows: the display name, else the name of the Action.
	Name string
}

// ours tells whether the entry is the Action of setup.
func (s *ActionState) ours(f FlowAction) bool {
	return s.ID != "" && f.RefType == management.ActionBindingReferenceByID && f.RefValue == s.ID
}

// others returns the Actions of the flow other than the one of setup, in order.
func (s *ActionState) others() []FlowAction {
	others := make([]FlowAction, 0, len(s.Flow))

	for _, f := range s.Flow {
		if !s.ours(f) {
			others = append(others, f)
		}
	}

	return others
}

// bound tells whether the Action of setup is in the flow.
func (s *ActionState) bound() bool {
	return len(s.others()) < len(s.Flow)
}

// last tells whether the Action of setup is bound once, as the last of the flow.
func (s *ActionState) last() bool {
	return s.bound() && len(s.others()) == len(s.Flow)-1 && s.ours(s.Flow[len(s.Flow)-1])
}

// position describes where the Action of setup is in the flow.
func (s *ActionState) position() string {
	for i := len(s.Flow) - 1; i >= 0; i-- {
		if s.ours(s.Flow[i]) {
			return fmt.Sprintf("%d of %d", i+1, len(s.Flow))
		}
	}

	return "not bound"
}

// writtenAPIs matches the list of APIs that OfflineAccessActionCode wrote.
var writtenAPIs = regexp.MustCompile(`const POPPY_APIS = (\[[^\]]*\]);`)

// apisOf returns the identifiers written into the code of an Action of setup.
// It returns none when the code has no list, such as code someone edited.
func apisOf(code string) []string {
	match := writtenAPIs.FindStringSubmatch(code)
	if match == nil {
		return nil
	}

	var identifiers []string
	if err := json.Unmarshal([]byte(match[1]), &identifiers); err != nil {
		return nil
	}

	return identifiers
}
