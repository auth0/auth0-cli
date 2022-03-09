package display

import "fmt"

type importView struct {
	Additions string
	Changes string
	Deletions string
	raw  interface{}
}

func (v *importView) AsTableHeader() []string {
	return []string{}
}

func (v *importView) AsTableRow() []string {
	return []string{}
}

func (v *importView) KeyValues() [][]string {
	return [][]string{
		{"ADDITIONS", v.Additions},
		{"CHANGES", v.Changes},
		{"DELETIONS", v.Deletions},
	}
}

func (v *importView) Object() interface{} {
	return v.raw
}

func (r *Renderer) Import(additions int, changes int, deletions int) {
	r.Heading()
	r.Result(&importView{
			Additions: fmt.Sprintf("%d", additions),
			Changes: fmt.Sprintf("%d", changes),
			Deletions: fmt.Sprintf("%d", deletions),
			raw: "",
		})
}
