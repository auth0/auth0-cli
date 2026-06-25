package display

import (
	"fmt"
	"strings"
	"time"

	management "github.com/auth0/go-auth0/v2/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type experimentView struct {
	ID                 string
	Name               string
	Description        string
	Status             string
	Valid              string
	FeatureFlagID      string
	AuthFlow           string
	AllocationStrategy string
	Allocations        string
	StartedAt          string
	CreatedAt          string
	UpdatedAt          string
	raw                interface{}
}

func (v *experimentView) AsTableHeader() []string {
	return []string{"ID", "Name", "Status", "Valid", "Auth Flow", "Started"}
}

func (v *experimentView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Status, v.Valid, v.AuthFlow, v.StartedAt}
}

func (v *experimentView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"STATUS", v.Status},
		{"VALID", v.Valid},
		{"FEATURE FLAG", ansi.Faint(v.FeatureFlagID)},
		{"AUTH FLOW", v.AuthFlow},
		{"ALLOCATION", v.AllocationStrategy},
	}
	if v.Description != "" {
		kvs = append(kvs, []string{"DESCRIPTION", v.Description})
	}
	if v.Allocations != "" {
		kvs = append(kvs, []string{"ALLOCATIONS", v.Allocations})
	}
	if v.StartedAt != "" {
		kvs = append(kvs, []string{"STARTED", v.StartedAt})
	}
	kvs = append(kvs,
		[]string{"CREATED", v.CreatedAt},
		[]string{"UPDATED", v.UpdatedAt},
	)
	return kvs
}

func (v *experimentView) Object() interface{} {
	return v.raw
}

func experimentStatus(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return ansi.Green(s)
	case "draft":
		return ansi.Yellow(s)
	case "paused":
		return ansi.Yellow(s)
	case "completed", "archived":
		return ansi.Faint(s)
	default:
		return s
	}
}

func formatAllocations(allocations []*management.AllocationItem) string {
	if len(allocations) == 0 {
		return ""
	}
	parts := make([]string, 0, len(allocations))
	for _, a := range allocations {
		var part string
		role := ""
		if a.GetIsControl() {
			role = " (control)"
		} else if a.GetIsFallback() {
			role = " (fallback)"
		}
		if a.GetWeight() > 0 {
			part = fmt.Sprintf("%s%s %.0f%%", a.GetVariationID(), role, a.GetWeight()*100)
		} else if a.GetSegmentID() != "" {
			part = fmt.Sprintf("%s%s → segment:%s", a.GetVariationID(), role, a.GetSegmentID())
		} else {
			part = a.GetVariationID() + role
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " / ")
}

func optionalTimeAgo(t *time.Time) string {
	if t == nil {
		return ""
	}
	return timeAgo(*t)
}

func makeExperimentViewFromListItem(e *management.ExperimentListItem) *experimentView {
	return &experimentView{
		ID:                 e.GetID(),
		Name:               e.GetName(),
		Description:        e.GetDescription(),
		Status:             experimentStatus(string(e.GetStatus())),
		Valid:              boolean(e.GetIsValid()),
		FeatureFlagID:      e.GetFeatureFlagID(),
		AuthFlow:           e.GetAuthenticationFlow(),
		AllocationStrategy: string(e.GetAllocationStrategy()),
		Allocations:        formatAllocations(e.GetAllocations()),
		StartedAt:          optionalTimeAgo(e.StartedAt),
		CreatedAt:          timeAgo(e.GetCreatedAt()),
		UpdatedAt:          timeAgo(e.GetUpdatedAt()),
		raw:                e,
	}
}

func makeExperimentViewFromGet(e *management.GetExperimentResponseContent) *experimentView {
	return &experimentView{
		ID:                 e.GetID(),
		Name:               e.GetName(),
		Description:        e.GetDescription(),
		Status:             experimentStatus(string(e.GetStatus())),
		Valid:              boolean(e.GetIsValid()),
		FeatureFlagID:      e.GetFeatureFlagID(),
		AuthFlow:           e.GetAuthenticationFlow(),
		AllocationStrategy: string(e.GetAllocationStrategy()),
		Allocations:        formatAllocations(e.GetAllocations()),
		StartedAt:          optionalTimeAgo(e.StartedAt),
		CreatedAt:          timeAgo(e.GetCreatedAt()),
		UpdatedAt:          timeAgo(e.GetUpdatedAt()),
		raw:                e,
	}
}

func makeExperimentViewFromCreate(e *management.CreateExperimentResponseContent) *experimentView {
	return &experimentView{
		ID:                 e.GetID(),
		Name:               e.GetName(),
		Description:        e.GetDescription(),
		Status:             experimentStatus(string(e.GetStatus())),
		Valid:              boolean(e.GetIsValid()),
		FeatureFlagID:      e.GetFeatureFlagID(),
		AuthFlow:           e.GetAuthenticationFlow(),
		AllocationStrategy: string(e.GetAllocationStrategy()),
		Allocations:        formatAllocations(e.GetAllocations()),
		CreatedAt:          timeAgo(e.GetCreatedAt()),
		UpdatedAt:          timeAgo(e.GetUpdatedAt()),
		raw:                e,
	}
}

func makeExperimentViewFromStatusUpdate(e *management.UpdateExperimentStatusResponseContent) *experimentView {
	return &experimentView{
		ID:        e.GetID(),
		Name:      e.GetName(),
		Status:    experimentStatus(string(e.GetStatus())),
		Valid:     boolean(e.GetIsValid()),
		UpdatedAt: timeAgo(e.GetUpdatedAt()),
		raw:       e,
	}
}

func (r *Renderer) ExperimentList(experiments []*management.ExperimentListItem) {
	r.Heading("experiments")
	if len(experiments) == 0 {
		r.EmptyState("experiments", "Use 'auth0 experiments create' to add one")
		return
	}
	var res []View
	for _, e := range experiments {
		res = append(res, makeExperimentViewFromListItem(e))
	}
	r.Results(res)
}

func (r *Renderer) ExperimentShow(e *management.GetExperimentResponseContent) {
	r.Heading("experiment")
	r.Result(makeExperimentViewFromGet(e))
}

func (r *Renderer) ExperimentCreate(e *management.CreateExperimentResponseContent) error {
	r.Heading("experiment created")
	r.Result(makeExperimentViewFromCreate(e))
	r.Newline()
	r.Infof("To validate this experiment, run: auth0 experiments validate %s", e.GetID())
	return nil
}

func (r *Renderer) ExperimentUpdate(e *management.UpdateExperimentResponseContent) error {
	r.Heading("experiment updated")
	view := &experimentView{
		ID:        e.GetID(),
		Name:      e.GetName(),
		Status:    experimentStatus(string(e.GetStatus())),
		Valid:     boolean(e.GetIsValid()),
		UpdatedAt: timeAgo(e.GetUpdatedAt()),
		raw:       e,
	}
	r.Result(view)
	return nil
}

func (r *Renderer) ExperimentStatusUpdate(e *management.UpdateExperimentStatusResponseContent) error {
	r.Heading(fmt.Sprintf("experiment %s", strings.ToLower(string(e.GetStatus()))))
	r.Result(makeExperimentViewFromStatusUpdate(e))
	r.Newline()
	switch e.GetStatus() {
	case "active":
		r.Infof("Experiment is now running. To pause it, run: auth0 experiments pause %s", e.GetID())
	case "paused":
		r.Infof("Experiment paused. To resume, run: auth0 experiments start %s", e.GetID())
	case "completed":
		r.Infof("Experiment completed. To archive it, run: auth0 experiments archive %s", e.GetID())
	}
	return nil
}

func (r *Renderer) ExperimentValidate(id string, v *management.ValidateExperimentResponseContent) {
	r.Heading("experiment validated")

	view := &experimentValidationView{id: id, result: v}
	r.Result(view)
	r.Newline()
	if v.GetIsValid() {
		r.Infof("Experiment is ready to start. Run: auth0 experiments start %s", id)
	} else {
		r.Warnf("Fix the validation errors above before starting the experiment.")
	}
}

// experimentValidationView renders the validate result.
type experimentValidationView struct {
	id     string
	result *management.ValidateExperimentResponseContent
}

func (v *experimentValidationView) AsTableHeader() []string { return nil }
func (v *experimentValidationView) AsTableRow() []string    { return nil }
func (v *experimentValidationView) Object() interface{}     { return v.result }

func (v *experimentValidationView) KeyValues() [][]string {
	kvs := [][]string{
		{"VALID", boolean(v.result.GetIsValid())},
	}
	for _, e := range v.result.GetErrors() {
		kvs = append(kvs, []string{ansi.Red("ERROR"), fmt.Sprintf("%s: %s", e.GetCode(), e.GetMessage())})
	}
	return kvs
}
