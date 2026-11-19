package display

import (
	"io"
	"regexp"
	"strings"

	"golang.org/x/term"
	"golang.org/x/text/width"

	"github.com/auth0/auth0-cli/internal/ansi"
)

const (
	// PoppyDefaultWidth is the width of a terminal whose size is not known, and
	// the width the right edge of a header is placed at when nothing is wrapped.
	poppyDefaultWidth = 80

	// PoppyMinWidth is the narrowest a wrapped text is made, however narrow the
	// terminal is.
	poppyMinWidth = 20
)

// Layout is how the text output of the Poppy commands is laid out for one
// writer.
type Layout struct {
	// Width is the width of the terminal. Zero means the output is not a terminal,
	// so nothing is wrapped and a script sees whole lines.
	Width int
	// Unicode allows the symbols that are not ASCII. Otherwise they have plain
	// ASCII fallbacks.
	Unicode bool
}

// LayoutOf works out the layout for a writer. A terminal gets its width (80
// columns when the size is not known) and the Unicode symbols. Any other writer
// is not wrapped, and gets the Unicode symbols only when colors are on.
func LayoutOf(w io.Writer) Layout {
	l := Layout{Unicode: ansi.Bold("x") != "x"}

	f, ok := w.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return l
	}

	l.Unicode = true
	l.Width = poppyDefaultWidth
	if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
		l.Width = width
	}

	return l
}

// lineWidth is the width to align things to the right edge at: the width of the
// terminal, else 80.
func (l Layout) lineWidth() int {
	if l.Width > 0 {
		return l.Width
	}

	return poppyDefaultWidth
}

// wrap breaks s into lines that fit the layout after an indent of the given
// width, on word boundaries. A text that fits, or any text when the layout does
// not wrap, stays one line as it is. It always returns at least one line.
func (l Layout) wrap(s string, indent int) []string {
	width := max(l.Width-indent, poppyMinWidth)

	if l.Width <= 0 || poppyCells(s) <= width {
		return []string{strings.TrimSpace(s)}
	}

	return wrapWords(s, width)
}

// wrapWords breaks s into lines of at most width cells on word boundaries. A word
// longer than a line is cut. It always returns at least one line.
func wrapWords(s string, width int) []string {
	var (
		lines []string
		line  string
		used  int
	)
	for word := range strings.FieldsSeq(s) {
		for poppyCells(word) > width {
			if used > 0 {
				lines, line, used = append(lines, line), "", 0
			}

			var head string
			head, word = splitCells(word, width)
			lines = append(lines, head)
		}

		cells := poppyCells(word)

		switch {
		case used == 0:
			line, used = word, cells
		case used+1+cells <= width:
			line, used = line+" "+word, used+1+cells
		default:
			lines, line, used = append(lines, line), word, cells
		}
	}

	return append(lines, line)
}

// splitCells cuts the longest start of s that fits in the given cells, and always
// takes at least one rune.
func splitCells(s string, cells int) (head, rest string) {
	used := 0

	for i, r := range s {
		used += poppyRuneCells(r)
		if used > cells && i > 0 {
			return s[:i], s[i:]
		}
	}

	return s, ""
}

// hang writes s wrapped to the layout after a prefix: the first line starts with
// the prefix and the other lines are indented to the width of its plain text, so
// the text sits in a column. Colored may hold the prefix with color codes; plain
// is the same prefix without them. Each line ends with a newline.
func (l Layout) hang(plain, colored, s string) string {
	pad := strings.Repeat(" ", poppyCells(plain))
	lines := l.wrap(s, poppyCells(plain))

	var b strings.Builder
	b.WriteString(colored)
	b.WriteString(lines[0])
	b.WriteString("\n")
	for _, line := range lines[1:] {
		b.WriteString(pad)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// header is a title in bold at the left edge and a note at the right edge, on one
// line that ends with a newline.
func (l Layout) header(left, right string) string {
	gap := l.lineWidth() - poppyCells(left) - poppyCells(right)

	return ansi.Bold(left) + strings.Repeat(" ", max(gap, 1)) + right + "\n"
}

// poppyMark is a symbol that starts a line of the output.
type poppyMark int

// The symbols of the output.
const (
	// MarkDone is a step that is done, or a value that is already right.
	markDone poppyMark = iota
	// MarkChanged is a value that is changed.
	markChanged
	// MarkAdded is something that is added.
	markAdded
	// MarkWarning is something to look at.
	markWarning
	// MarkFailed is a step that failed.
	markFailed
	// MarkSkipped is a step that did not run.
	markSkipped
	// MarkPlain is a line without a symbol, kept in line with the ones that have one.
	markPlain
)

// mark returns the symbol of a mark, and what to print for it: colored when the
// output has colors. Without Unicode the symbol is plain ASCII.
func (l Layout) mark(m poppyMark) (symbol, colored string) {
	switch m {
	case markDone:
		symbol = l.pick("✓", "ok")

		return symbol, ansi.Green(symbol)
	case markChanged:
		return "~", ansi.Yellow("~")
	case markAdded:
		return "+", ansi.Green("+")
	case markWarning:
		return "!", ansi.Yellow("!")
	case markFailed:
		symbol = l.pick("✗", "x")

		return symbol, ansi.Red(symbol)
	case markSkipped:
		return "-", ansi.Faint("-")
	default:
		return " ", " "
	}
}

// markWidth is the width of the widest symbol, so lines that start with
// different marks can be padded to line up.
func (l Layout) markWidth() int {
	if l.Unicode {
		return 1
	}

	return 2
}

// arrow separates the value before a change from the value after it.
func (l Layout) arrow() string { return l.pick("→", "->") }

// alert is the sign that heads a note about wide effects.
func (l Layout) alert() string { return l.pick("⚠", "!") }

func (l Layout) pick(unicode, ascii string) string {
	if l.Unicode {
		return unicode
	}

	return ascii
}

// poppyEntry is a line of a block: a mark, a label in a column, then a value that
// wraps in its own column.
type poppyEntry struct {
	mark  poppyMark
	label string
	value string
}

// entries writes the lines of a block with the labels padded to one width. An
// entry without a label puts its value right after the mark, and with no mark
// either it only indents the value. Each line ends with a newline.
func (l Layout) entries(entries []poppyEntry) string {
	labelWidth := 0
	for _, e := range entries {
		labelWidth = max(labelWidth, poppyCells(e.label))
	}

	var b strings.Builder

	for _, e := range entries {
		symbol, colored := l.mark(e.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-poppyCells(symbol), 0))

		plain, shown := "  "+symbol+pad+" ", "  "+colored+pad+" "
		if e.mark == markPlain && e.label == "" {
			plain, shown = "  ", "  "
		}
		if e.label != "" {
			label := e.label + strings.Repeat(" ", labelWidth-poppyCells(e.label)+2)
			plain, shown = plain+label, shown+label
		}

		b.WriteString(l.hang(plain, shown, e.value))
	}

	return b.String()
}

// ansiSequence matches the escape sequences that color text. They take no room on
// the screen.
var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// poppyCells is the number of cells that s takes on the screen: the color
// sequences take none, and a character that is wide or fullwidth in East Asian
// text takes two. Combining marks and emoji are not special cased, so they count
// as one cell each, as they do for the other width math here.
func poppyCells(s string) int {
	cells := 0

	for _, r := range ansiSequence.ReplaceAllString(s, "") {
		cells += poppyRuneCells(r)
	}

	return cells
}

// poppyRuneCells is the number of cells that one character takes, see poppyCells.
func poppyRuneCells(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}
