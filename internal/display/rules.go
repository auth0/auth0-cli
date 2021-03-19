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
	return []string{v.ID, v.Name, strconv.FormatBool(v.Enabled), fmt.Sprintf("%d", v.Order)}
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

func (r *Renderer) RulesList(ruleList *management.RuleList) {
	r.Heading(ansi.Bold(r.Tenant), "rules\n")
	var res []View

	//@TODO Provide sort options via flags
	sort.Slice(ruleList.Rules, func(i, j int) bool {
		return ruleList.Rules[i].GetOrder() < ruleList.Rules[j].GetOrder()
	})

	for _, rule := range ruleList.Rules {
		res = append(res, &ruleView{
			Name:    *rule.Name,
			ID:      *rule.ID,
			Enabled: *rule.Enabled,
			Order:   *rule.Order,
		})
	}

	r.Results(res)

}

func (r *Renderer) RulesCreate(rule *management.Rule) {
	r.Heading(ansi.Bold(r.Tenant), "rule created\n")

	v := &ruleView{
		Name:    rule.GetName(),
		ID:      rule.GetID(),
		Enabled: rule.GetEnabled(),
		Order:   rule.GetOrder(),
		Script:  rule.GetScript(),

		raw: rule,
	}

	r.Result(v)

	r.Newline()

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s: To edit this rule, do `auth0 rules update %s`",
		ansi.Faint("Hint"),
		rule.GetID(),
	)

	r.Infof("%s: You might wanna try `auth0 test login",
		ansi.Faint("Hint"),
	)
}
