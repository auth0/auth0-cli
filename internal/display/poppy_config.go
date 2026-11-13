package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/text"
)

// PoppyConfigResult writes what the config command did to the file of the inputs:
// that it was written or left as it was, and what to do next. A run that asked
// questions has printed its title, and its answers about hints stay above the
// result; headerShown tells that. Unhinted is the number of scopes that were left
// without hints on purpose.
func (r *Renderer) PoppyConfigResult(in setup.Inputs, unchanged, headerShown bool, unhinted int) {
	r.poppyWrite(func(l Layout) string { return poppyConfigText(l, in, unchanged, headerShown, unhinted) })
}

func poppyConfigText(l Layout, in setup.Inputs, unchanged, headerShown bool, unhinted int) string {
	result := "written"
	if unchanged {
		result = "unchanged"
	}

	var next [][2]string
	if unhinted > 0 {
		next = append(next, [2]string{
			"auth0 poppy config --hint <scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>",
			fmt.Sprintf("to set the hints of %d %s.", unhinted, poppyScopesNoun(unhinted)),
		})
	}

	var b strings.Builder

	// The title of the run is above, once questions were asked.
	if headerShown {
		b.WriteString("\n")

		if answers := poppyHintsText(l, in); answers != "" {
			b.WriteString(answers)
			b.WriteString("\n")
		}
	}

	b.WriteString(l.entries([]poppyEntry{{mark: markDone, label: text.Line(in.Out), value: result}}))

	if len(next) > 0 {
		b.WriteString("\n")
		b.WriteString(poppyNext(l, next...))
	}

	return b.String()
}
