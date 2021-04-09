package display

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type ruleView struct {
	Name    string
	Enabled bool
	ID      string
	Order   int
	Script  string

	raw interface{}
}

func (v *ruleView) AsTableHeader() []string {
	return []string{"Id", "Name", "Enabled", "Order"}
}

func (v *ruleView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, strconv.FormatBool(v.Enabled), fmt.Sprintf("%d", v.Order)}
}

func (v *ruleView) KeyValues() [][]string {
	return [][]string{
		[]string{"NAME", v.Name},
		[]string{"ID", v.ID},
		[]string{"ENABLED", strconv.FormatBool(v.Enabled)},
		[]string{"SCRIPT", v.Script},
	}
}

func (v *ruleView) Object() interface{} {
	return v.raw
}

func (r *Renderer) RulesList(rules []*management.Rule) {
	resource := "rules"

	r.Heading(resource)

	if len(rules) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 rules create' to add one")
		return
	}

	var res []View

	//@TODO Provide sort options via flags
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].GetOrder() < rules[j].GetOrder()
	})

	for _, rule := range rules {
		res = append(res, &ruleView{
			Name:    *rule.Name,
			ID:      *rule.ID,
			Enabled: *rule.Enabled,
			Order:   *rule.Order,
		})
	}

	r.Results(res)

}

func (r *Renderer) RuleCreate(rule *management.Rule) {
	r.Heading("rule created")
	r.Result(makeRuleView(rule))
	r.Newline()

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s To edit this rule, do 'auth0 rules update %s'",
		ansi.Faint("Hint:"),
		rule.GetID(),
	)

	r.Infof("%s You might wanna try 'auth0 test login'",
		ansi.Faint("Hint:"),
	)
}

func (r *Renderer) RuleUpdate(rule *management.Rule) {
	r.Heading("rule updated")
	r.Result(makeRuleView(rule))
}

func (r *Renderer) RuleShow(rule *management.Rule) {
	r.Heading("rule")
	r.Result(makeRuleView(rule))
}

func makeRuleView(rule *management.Rule) *ruleView {
	return &ruleView{
		Name:    rule.GetName(),
		ID:      rule.GetID(),
		Enabled: rule.GetEnabled(),
		Order:   rule.GetOrder(),
		Script:  rule.GetScript(),

		raw: rule,
	}
}
