package display

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

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

// PoppyLayout is how the text output of the Poppy commands is laid out for one
// writer.
type PoppyLayout struct {
	// Width is the width of the terminal. Zero means the output is not a terminal,
	// so nothing is wrapped and a script sees whole lines.
	Width int
	// Unicode allows the symbols that are not ASCII. Otherwise they have plain
	// ASCII fallbacks.
	Unicode bool
}

// PoppyLayoutOf works out the layout for a writer. A terminal gets its width (80
// columns when the size is not known) and the Unicode symbols. Any other writer
// is not wrapped, and gets the Unicode symbols only when colors are on.
func PoppyLayoutOf(w io.Writer) PoppyLayout {
	l := PoppyLayout{Unicode: ansi.Bold("x") != "x"}

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

// PoppyCanErase tells whether text written to w can be taken back off the screen:
// w is a terminal, and its type is not the dumb one that ignores cursor movement.
// Whether colors are on does not matter.
func PoppyCanErase(w io.Writer) bool {
	return PoppyLayoutOf(w).Width > 0 && os.Getenv("TERM") != "dumb"
}

// lineWidth is the width to align things to the right edge at: the width of the
// terminal, else 80.
func (l PoppyLayout) lineWidth() int {
	if l.Width > 0 {
		return l.Width
	}

	return poppyDefaultWidth
}

// wrap breaks s into lines that fit the layout after an indent of the given
// width, on word boundaries. A text that fits, or any text when the layout does
// not wrap, stays one line as it is. It always returns at least one line.
func (l PoppyLayout) wrap(s string, indent int) []string {
	width := max(l.Width-indent, poppyMinWidth)

	if l.Width <= 0 || utf8.RuneCountInString(s) <= width {
		return []string{strings.TrimSpace(s)}
	}

	return wrapWords(s, width)
}

// wrapWords breaks s into lines of at most width characters on word boundaries. A
// word longer than a line is cut. It always returns at least one line.
func wrapWords(s string, width int) []string {
	var (
		lines []string
		line  []rune
	)
	for _, word := range strings.Fields(s) {
		w := []rune(word)

		for len(w) > width {
			if len(line) > 0 {
				lines, line = append(lines, string(line)), nil
			}
			lines, w = append(lines, string(w[:width])), w[width:]
		}

		switch {
		case len(line) == 0:
			line = w
		case len(line)+1+len(w) <= width:
			line = append(append(line, ' '), w...)
		default:
			lines, line = append(lines, string(line)), w
		}
	}

	return append(lines, string(line))
}

// hang writes s wrapped to the layout after a prefix: the first line starts with
// the prefix and the other lines are indented to the width of its plain text, so
// the text sits in a column. Colored may hold the prefix with color codes; plain
// is the same prefix without them. Each line ends with a newline.
func (l PoppyLayout) hang(plain, colored, s string) string {
	pad := strings.Repeat(" ", utf8.RuneCountInString(plain))
	lines := l.wrap(s, utf8.RuneCountInString(plain))

	var b strings.Builder
	b.WriteString(colored + lines[0] + "\n")
	for _, line := range lines[1:] {
		b.WriteString(pad + line + "\n")
	}

	return b.String()
}

// header is a title in bold at the left edge and a note at the right edge, on one
// line that ends with a newline.
func (l PoppyLayout) header(left, right string) string {
	gap := l.lineWidth() - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)

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
func (l PoppyLayout) mark(m poppyMark) (symbol, colored string) {
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
func (l PoppyLayout) markWidth() int {
	if l.Unicode {
		return 1
	}

	return 2
}

// arrow separates the value before a change from the value after it.
func (l PoppyLayout) arrow() string { return l.pick("→", "->") }

// alert is the sign that heads a note about wide effects.
func (l PoppyLayout) alert() string { return l.pick("⚠", "!") }

func (l PoppyLayout) pick(unicode, ascii string) string {
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
func (l PoppyLayout) entries(entries []poppyEntry) string {
	labelWidth := 0
	for _, e := range entries {
		labelWidth = max(labelWidth, utf8.RuneCountInString(e.label))
	}

	var b strings.Builder

	for _, e := range entries {
		symbol, colored := l.mark(e.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-utf8.RuneCountInString(symbol), 0))

		plain, shown := "  "+symbol+pad+" ", "  "+colored+pad+" "
		if e.mark == markPlain && e.label == "" {
			plain, shown = "  ", "  "
		}
		if e.label != "" {
			label := e.label + strings.Repeat(" ", labelWidth-utf8.RuneCountInString(e.label)+2)
			plain, shown = plain+label, shown+label
		}

		b.WriteString(l.hang(plain, shown, e.value))
	}

	return b.String()
}

// ansiSequence matches the escape sequences that color text. They take no room on
// the screen.
var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// poppyBlock is a part of the screen that can be taken back. Everything written to
// it goes to the writer and is counted in screen rows, so that erase can clear
// exactly those rows with cursor movement and nothing else. It never relies on a
// saved cursor position, which a scrolling screen invalidates.
//
// Text written to a block must end each line with a newline: a last line without
// one is not counted.
type poppyBlock struct {
	w     io.Writer
	width int
	can   bool
	rows  int
	// Tail is the text after the last newline written.
	tail string
}

// newPoppyBlock starts an empty block on w. The layout gives the width at which
// long lines wrap on the screen, and can tells whether cursor movement works
// there, see PoppyCanErase. When it does not, erase leaves the text in place.
func newPoppyBlock(w io.Writer, l PoppyLayout, can bool) *poppyBlock {
	return &poppyBlock{w: w, width: l.Width, can: can}
}

// Write writes p to the writer and counts the rows it takes.
func (b *poppyBlock) Write(p []byte) (int, error) {
	n, err := b.w.Write(p)

	lines := strings.Split(b.tail+string(p[:n]), "\n")
	for _, line := range lines[:len(lines)-1] {
		b.rows += poppyLineRows(line, b.width)
	}
	b.tail = lines[len(lines)-1]

	return n, err
}

// add counts rows that something else wrote in the place of the block, such as a
// prompt that was answered and left them on the screen.
func (b *poppyBlock) add(rows int) {
	b.rows += max(rows, 0)
}

// erase takes the block off the screen. The cursor must be at the start of the
// row below the block. It moves up over the rows and clears from there to the end
// of the screen. A block that cannot be erased is only forgotten, so the next one
// counts its own rows.
func (b *poppyBlock) erase() error {
	rows := b.rows
	b.rows, b.tail = 0, ""

	if !b.can || rows == 0 {
		return nil
	}

	_, err := fmt.Fprintf(b.w, "\r\x1b[%dA\x1b[J", rows)

	return err
}

// poppyLineRows is the number of screen rows that a line takes in a terminal of
// the given width. A line of exactly the width takes one row, and so does an empty
// line. A width of zero means the line is not wrapped.
func poppyLineRows(line string, width int) int {
	if width <= 0 {
		return 1
	}

	cells := utf8.RuneCountInString(ansiSequence.ReplaceAllString(line, ""))

	return max((cells+width-1)/width, 1)
}
