package display

import (
	"gopkg.in/auth0.v5/management"
)

type userBlockView struct {
	Identifier string
	IP         string
}

func (v *userBlockView) AsTableHeader() []string {
	return []string{"Identifier", "IP"}
}

func (v *userBlockView) AsTableRow() []string {
	return []string{v.Identifier, v.IP}
}

func (v *userBlockView) KeyValues() [][]string {
	return [][]string{
		[]string{"Identifier", v.Identifier},
		[]string{"IP", v.IP},
	}
}

func (r *Renderer) UserBlocksList(userBlocks []*management.UserBlock) {
	resource := "user blocks"

	r.Heading(resource)

	if len(userBlocks) == 0 {
		r.EmptyState(resource)
		return
	}

	var res []View

	for _, userBlock := range userBlocks {
		res = append(res, &userBlockView{
			Identifier: *userBlock.Identifier,
			IP:         *userBlock.IP,
		})
	}

	r.Results(res)

}
