package display

import (
	"fmt"
	"sort"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type ruleView struct {
	rule management.Rule
}

func (v *ruleView) AsTableHeader() []string {
	return []string{"Id", "Name", "Status", "Order"}
}

func (v *ruleView) AsTableRow() []string {
	return []string{*v.rule.ID, *v.rule.Name, isEnabled(*v.rule.Enabled), fmt.Sprintf("%d", *v.rule.Order)}
}

func isEnabled(value bool) string {
	if value {
		return "True"
	}
	return "False"
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
			rule: *rule,
		})
	}

	r.Results(res)

}
