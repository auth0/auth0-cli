package display

import (
	"encoding/json"
	"strings"

	management "github.com/auth0/go-auth0/v2/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type segmentView struct {
	ID          string
	Name        string
	Description string
	Type        string
	Rules       string
	CreatedAt   string
	UpdatedAt   string
	raw         interface{}
}

func (v *segmentView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Rules", "Updated"}
}

func (v *segmentView) AsTableRow() []string {
	rules := v.Rules
	if len(rules) > 50 {
		rules = rules[:47] + "..."
	}
	return []string{ansi.Faint(v.ID), v.Name, v.Type, rules, v.UpdatedAt}
}

func (v *segmentView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
	}
	if v.Description != "" {
		kvs = append(kvs, []string{"DESCRIPTION", v.Description})
	}
	kvs = append(kvs,
		[]string{"RULES", v.Rules},
		[]string{"CREATED", v.CreatedAt},
		[]string{"UPDATED", v.UpdatedAt},
	)
	return kvs
}

func (v *segmentView) Object() interface{} {
	return v.raw
}

func formatRules(rules []*management.SegmentRule) string {
	if len(rules) == 0 {
		return "[]"
	}
	b, err := json.Marshal(rules)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func makeSegmentView(s *management.Segment) *segmentView {
	return &segmentView{
		ID:          s.GetID(),
		Name:        s.GetName(),
		Description: s.GetDescription(),
		Type:        strings.ToLower(string(s.GetType())),
		Rules:       formatRules(s.GetRules()),
		CreatedAt:   timeAgo(s.GetCreatedAt()),
		UpdatedAt:   timeAgo(s.GetUpdatedAt()),
		raw:         s,
	}
}

func makeSegmentViewFromGet(s *management.GetSegmentResponseContent) *segmentView {
	return &segmentView{
		ID:          s.GetID(),
		Name:        s.GetName(),
		Description: s.GetDescription(),
		Type:        strings.ToLower(string(s.GetType())),
		Rules:       formatRules(s.GetRules()),
		CreatedAt:   timeAgo(s.GetCreatedAt()),
		UpdatedAt:   timeAgo(s.GetUpdatedAt()),
		raw:         s,
	}
}

func (r *Renderer) SegmentList(segments []*management.Segment) {
	r.Heading("segments")
	if len(segments) == 0 {
		r.EmptyState("segments", "Use 'auth0 segments create' to add one")
		return
	}
	var res []View
	for _, s := range segments {
		res = append(res, makeSegmentView(s))
	}
	r.Results(res)
}

func (r *Renderer) SegmentShow(s *management.GetSegmentResponseContent) {
	r.Heading("segment")
	r.Result(makeSegmentViewFromGet(s))
}

func (r *Renderer) SegmentCreate(s *management.CreateSegmentResponseContent) error {
	r.Heading("segment created")
	view := &segmentView{
		ID:          s.GetID(),
		Name:        s.GetName(),
		Description: s.GetDescription(),
		Type:        strings.ToLower(string(s.GetType())),
		Rules:       formatRules(s.GetRules()),
		CreatedAt:   timeAgo(s.GetCreatedAt()),
		UpdatedAt:   timeAgo(s.GetUpdatedAt()),
		raw:         s,
	}
	r.Result(view)
	r.Newline()
	r.Infof("To use this segment in an experiment, run: auth0 experiments create")
	return nil
}

func (r *Renderer) SegmentUpdate(s *management.UpdateSegmentResponseContent) error {
	r.Heading("segment updated")
	view := &segmentView{
		ID:        s.GetID(),
		Name:      s.GetName(),
		Type:      strings.ToLower(string(s.GetType())),
		Rules:     formatRules(s.GetRules()),
		UpdatedAt: timeAgo(s.GetUpdatedAt()),
		raw:       s,
	}
	r.Result(view)
	return nil
}
