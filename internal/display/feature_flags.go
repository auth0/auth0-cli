package display

import (
	"encoding/json"
	"fmt"
	"strings"

	management "github.com/auth0/go-auth0/v2/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type featureFlagView struct {
	ID          string
	Name        string
	Description string
	Type        string
	Status      string
	Parameters  string
	CreatedAt   string
	UpdatedAt   string
	raw         interface{}
}

func (v *featureFlagView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Status", "Updated"}
}

func (v *featureFlagView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Type, v.Status, v.UpdatedAt}
}

func (v *featureFlagView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"STATUS", v.Status},
	}
	if v.Description != "" {
		kvs = append(kvs, []string{"DESCRIPTION", v.Description})
	}
	if v.Parameters != "" {
		kvs = append(kvs, []string{"PARAMETERS", v.Parameters})
	}
	kvs = append(kvs,
		[]string{"CREATED", v.CreatedAt},
		[]string{"UPDATED", v.UpdatedAt},
	)
	return kvs
}

func (v *featureFlagView) Object() interface{} {
	return v.raw
}

func featureFlagStatus(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return ansi.Green(s)
	case "archived":
		return ansi.Faint(s)
	default:
		return ansi.Yellow(s)
	}
}

func makeFeatureFlagView(ff *management.FeatureFlag) *featureFlagView {
	params := ""
	if ff.Parameters != nil {
		if b, err := json.Marshal(ff.Parameters); err == nil {
			params = string(b)
		}
	}
	return &featureFlagView{
		ID:          ff.GetID(),
		Name:        ff.GetName(),
		Description: ff.GetDescription(),
		Type:        string(ff.GetType()),
		Status:      featureFlagStatus(string(ff.GetStatus())),
		Parameters:  params,
		CreatedAt:   timeAgo(ff.GetCreatedAt()),
		UpdatedAt:   timeAgo(ff.GetUpdatedAt()),
		raw:         ff,
	}
}

func makeFeatureFlagViewFromGet(ff *management.GetFeatureFlagResponseContent) *featureFlagView {
	params := ""
	if ff.Parameters != nil {
		if b, err := json.Marshal(ff.Parameters); err == nil {
			params = string(b)
		}
	}
	return &featureFlagView{
		ID:          ff.GetID(),
		Name:        ff.GetName(),
		Description: ff.GetDescription(),
		Type:        string(ff.GetType()),
		Status:      featureFlagStatus(string(ff.GetStatus())),
		Parameters:  params,
		CreatedAt:   timeAgo(ff.GetCreatedAt()),
		UpdatedAt:   timeAgo(ff.GetUpdatedAt()),
		raw:         ff,
	}
}

func (r *Renderer) FeatureFlagList(flags []*management.FeatureFlag) {
	r.Heading("feature flags")
	if len(flags) == 0 {
		r.EmptyState("feature flags", "Use 'auth0 feature-flags create' to add one")
		return
	}
	var res []View
	for _, ff := range flags {
		res = append(res, makeFeatureFlagView(ff))
	}
	r.Results(res)
}

func (r *Renderer) FeatureFlagShow(ff *management.GetFeatureFlagResponseContent) {
	r.Heading("feature flag")
	r.Result(makeFeatureFlagViewFromGet(ff))
}

func (r *Renderer) FeatureFlagCreate(ff *management.CreateFeatureFlagResponseContent) error {
	r.Heading("feature flag created")
	// CreateFeatureFlagResponseContent has the same fields — project through a FeatureFlag for display.
	view := &featureFlagView{
		ID:          ff.GetID(),
		Name:        ff.GetName(),
		Description: ff.GetDescription(),
		Type:        string(ff.GetType()),
		Status:      featureFlagStatus(string(ff.GetStatus())),
		CreatedAt:   timeAgo(ff.GetCreatedAt()),
		UpdatedAt:   timeAgo(ff.GetUpdatedAt()),
		raw:         ff,
	}
	if ff.Parameters != nil {
		if b, err := json.Marshal(ff.Parameters); err == nil {
			view.Parameters = string(b)
		}
	}
	r.Result(view)
	r.Newline()
	r.Infof("To manage variations, run: auth0 feature-flags variations list %s", ff.GetID())
	return nil
}

func (r *Renderer) FeatureFlagUpdate(ff *management.UpdateFeatureFlagResponseContent) error {
	r.Heading("feature flag updated")
	view := &featureFlagView{
		ID:          ff.GetID(),
		Name:        ff.GetName(),
		Description: ff.GetDescription(),
		Type:        string(ff.GetType()),
		Status:      featureFlagStatus(string(ff.GetStatus())),
		UpdatedAt:   timeAgo(ff.GetUpdatedAt()),
		raw:         ff,
	}
	r.Result(view)
	return nil
}

// variationView -----------------------------------------------------------------

type variationView struct {
	ID            string
	FeatureFlagID string
	Name          string
	Description   string
	Overrides     string
	CreatedAt     string
	UpdatedAt     string
	raw           interface{}
}

func (v *variationView) AsTableHeader() []string {
	return []string{"ID", "Name", "Overrides", "Updated"}
}

func (v *variationView) AsTableRow() []string {
	overrides := v.Overrides
	if len(overrides) > 60 {
		overrides = overrides[:57] + "..."
	}
	return []string{ansi.Faint(v.ID), v.Name, overrides, v.UpdatedAt}
}

func (v *variationView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"FEATURE FLAG", ansi.Faint(v.FeatureFlagID)},
		{"NAME", v.Name},
	}
	if v.Description != "" {
		kvs = append(kvs, []string{"DESCRIPTION", v.Description})
	}
	kvs = append(kvs,
		[]string{"OVERRIDES", v.Overrides},
		[]string{"CREATED", v.CreatedAt},
		[]string{"UPDATED", v.UpdatedAt},
	)
	return kvs
}

func (v *variationView) Object() interface{} {
	return v.raw
}

func formatOverrides(overrides management.VariationOverridesMap) string {
	if len(overrides) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(overrides))
	for k, v := range overrides {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v.GetValue()))
	}
	return strings.Join(parts, ", ")
}

func makeVariationView(v *management.Variation) *variationView {
	return &variationView{
		ID:            v.GetID(),
		FeatureFlagID: v.GetFeatureFlagID(),
		Name:          v.GetName(),
		Description:   v.GetDescription(),
		Overrides:     formatOverrides(v.GetOverrides()),
		CreatedAt:     timeAgo(v.GetCreatedAt()),
		UpdatedAt:     timeAgo(v.GetUpdatedAt()),
		raw:           v,
	}
}

func makeVariationViewFromGet(v *management.GetVariationResponseContent) *variationView {
	return &variationView{
		ID:            v.GetID(),
		FeatureFlagID: v.GetFeatureFlagID(),
		Name:          v.GetName(),
		Description:   v.GetDescription(),
		Overrides:     formatOverrides(v.GetOverrides()),
		CreatedAt:     timeAgo(v.GetCreatedAt()),
		UpdatedAt:     timeAgo(v.GetUpdatedAt()),
		raw:           v,
	}
}

func (r *Renderer) VariationList(variations []*management.Variation) {
	r.Heading("variations")
	if len(variations) == 0 {
		r.EmptyState("variations", "Use 'auth0 feature-flags variations create <feature-flag-id>' to add one")
		return
	}
	var res []View
	for _, v := range variations {
		res = append(res, makeVariationView(v))
	}
	r.Results(res)
}

func (r *Renderer) VariationShow(v *management.GetVariationResponseContent) {
	r.Heading("variation")
	r.Result(makeVariationViewFromGet(v))
}

func (r *Renderer) VariationCreate(v *management.CreateVariationResponseContent) error {
	r.Heading("variation created")
	view := &variationView{
		ID:            v.GetID(),
		FeatureFlagID: v.GetFeatureFlagID(),
		Name:          v.GetName(),
		Description:   v.GetDescription(),
		Overrides:     formatOverrides(v.GetOverrides()),
		CreatedAt:     timeAgo(v.GetCreatedAt()),
		UpdatedAt:     timeAgo(v.GetUpdatedAt()),
		raw:           v,
	}
	r.Result(view)
	return nil
}

func (r *Renderer) VariationUpdate(v *management.UpdateVariationResponseContent) error {
	r.Heading("variation updated")
	view := &variationView{
		ID:          v.GetID(),
		Name:        v.GetName(),
		Description: v.GetDescription(),
		Overrides:   formatOverrides(v.GetOverrides()),
		UpdatedAt:   timeAgo(v.GetUpdatedAt()),
		raw:         v,
	}
	r.Result(view)
	return nil
}
