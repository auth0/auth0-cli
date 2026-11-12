package display

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/text"
)

// ScopeListMarks is the width of what starts each row of a list to choose from
// several: the focus mark, the checkbox and the gaps around them.
const ScopeListMarks = 7

// ScopeChoice is a scope in a list to choose from.
type ScopeChoice struct {
	Name        string
	Description string
	// Hints are the hints of the scope, nil when they are not known.
	Hints *config.Hints
}

// poppyHintNote says what the hints of a scope are, for a list of scopes: what the
// scope changes and whether it touches personal data.
func poppyHintNote(hints *config.Hints) string {
	if hints == nil {
		return ""
	}

	note := "[changes: " + text.Line(hints.Changes)
	if hints.PersonalData {
		note += ", personal data"
	}

	return note + "]"
}

// ScopeRows lays out the scopes of a list to choose from, one row each: the name,
// the description in a column, and the hints when they are known. A description
// that does not fit the width of the terminal is cut with an ellipsis, so no row
// wraps.
func (s *PromptScreen) ScopeRows(scopes []ScopeChoice) []string {
	nameWidth := 0
	for _, scope := range scopes {
		nameWidth = max(nameWidth, poppyCells(text.Line(scope.Name)))
	}

	// The room taken by the focus mark, the checkbox and the gaps.
	room := s.layout.lineWidth() - ScopeListMarks - nameWidth - 2

	rows := make([]string, len(scopes))

	for i, scope := range scopes {
		name := text.Line(scope.Name)
		note := poppyHintNote(scope.Hints)

		description := text.Line(scope.Description)
		if note != "" {
			description = text.Cap(description, max(room-poppyCells(note)-1, poppyMinWidth/2)) + " " + note
		} else {
			description = text.Cap(description, max(room, poppyMinWidth/2))
		}

		rows[i] = name + strings.Repeat(" ", nameWidth-poppyCells(name)+2) + description
	}

	return rows
}
