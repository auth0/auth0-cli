package display

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/auth0"
)

type importView struct {
	Resource  string
	Additions string
	Changes   string
	Deletions string
	raw       interface{}
}

func (v *importView) AsTableHeader() []string {
	return []string{"RESOURCE", "ADDITIONS", "CHANGES", "DELETIONS"}
}

func (v *importView) AsTableRow() []string {
	return []string{v.Resource, v.Additions, v.Changes, v.Deletions}
}

func (v *importView) KeyValues() [][]string {
	return [][]string{}
}

func (v *importView) Object() interface{} {
	return v.raw
}

func (r *Renderer) Import(changes []*auth0.ImportChanges) {
	r.Heading("import sucessful")

	var res []View
	for _, change := range changes {
		res = append(res, makeImportView(change))
	}

	r.Results(res)
}

func makeImportView(change *auth0.ImportChanges) *importView {
	return &importView{
			Resource: change.Resource,
			Additions: fmt.Sprintf("%d", change.Creates),
			Changes: fmt.Sprintf("%d", change.Updates),
			Deletions: fmt.Sprintf("%d", change.Deletes),
			raw: change,
		}
}
