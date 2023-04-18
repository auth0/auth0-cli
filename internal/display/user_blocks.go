package display

import (
	"github.com/auth0/go-auth0/management"
)

type userBlockView struct {
	Identifier string
	IP         string
	raw        interface{}
}

func (v *userBlockView) AsTableHeader() []string {
	return []string{"Identifier", "IP"}
}

func (v *userBlockView) AsTableRow() []string {
	return []string{v.Identifier, v.IP}
}

func (v *userBlockView) KeyValues() [][]string {
	return [][]string{
		{"Identifier", v.Identifier},
		{"IP", v.IP},
	}
}

func (v *userBlockView) Object() interface{} {
	return v.raw
}

func (r *Renderer) UserBlocksList(userBlocks []*management.UserBlock) {
	resource := "user blocks"

	r.Heading(resource)

	if len(userBlocks) == 0 {
		if r.Format == OutputFormatJSON {
			r.JSONResult([]interface{}{})
			return
		}
		r.EmptyState(resource)
		return
	}

	var res []View

	for _, userBlock := range userBlocks {
		res = append(res, &userBlockView{
			Identifier: *userBlock.Identifier,
			IP:         *userBlock.IP,
			raw:        userBlock,
		})
	}

	r.Results(res)
}
