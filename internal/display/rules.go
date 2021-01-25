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
}

func (v *ruleView) AsTableHeader() []string {
	return []string{"Id", "Name", "Enabled", "Order"}
}

func (v *ruleView) AsTableRow() []string {
	return []string{v.ID, v.Name, strconv.FormatBool(v.Enabled), fmt.Sprintf("%d", v.Order)}
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
