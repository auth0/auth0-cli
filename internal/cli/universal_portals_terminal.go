package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/briandowns/spinner"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/portals"
)

// The glyphs of the output. The left rail ties the lines of one run together, and each kind of
// line starts with its own mark.
const (
	railGlyph    = "│"
	stepGlyph    = "◇"
	successGlyph = "◆"
	infoGlyph    = "●"
	warnGlyph    = "▲"
	errorGlyph   = "■"
	endGlyph     = "└"
)

// portalsStyle paints the output. Tests use a style that paints nothing.
type portalsStyle struct {
	bold   func(string) string
	faint  func(string) string
	green  func(string) string
	yellow func(string) string
	red    func(string) string
	blue   func(string) string
	cyan   func(string) string
	// The link makes the text a link to the URL where the terminal supports it.
	link func(url, text string) string
}

// ansiPortalsStyle paints with the colors of the CLI, which follow --no-color and the terminal.
func ansiPortalsStyle() portalsStyle {
	return portalsStyle{
		bold:   ansi.Bold,
		faint:  ansi.Faint,
		green:  ansi.Green,
		yellow: ansi.Yellow,
		red:    ansi.BrightRed,
		blue:   ansi.Blue,
		cyan:   ansi.Cyan,
		link:   ansi.Hyperlink,
	}
}

// plainPortalsStyle paints nothing.
func plainPortalsStyle() portalsStyle {
	plain := func(text string) string { return text }

	return portalsStyle{
		bold: plain, faint: plain, green: plain, yellow: plain, red: plain, blue: plain, cyan: plain,
		link: func(_, text string) string { return text },
	}
}

// portalsTerminal shows the progress of setup and audit on the terminal. It is the only code that
// writes the human output of the portals commands, and it writes to the stream for messages, so
// the result of a command stays alone on the stream for results.
type portalsTerminal struct {
	out   io.Writer
	style portalsStyle
	// When silent, nothing is drawn: agents and JSON get only the result.
	silent bool
	// When animate is set, a spinner shows while work runs, which needs a terminal.
	animate bool
	// The spinner already drew the blank line that comes before the next line when spinnerGap is set.
	spinnerGap bool
}

var _ portals.Reporter = (*portalsTerminal)(nil)

func newPortalsTerminal(out io.Writer, style portalsStyle, silent, animate bool) *portalsTerminal {
	return &portalsTerminal{out: out, style: style, silent: silent, animate: animate}
}

func (t *portalsTerminal) println(line string) {
	_, _ = fmt.Fprintln(t.out, line)
}

func (t *portalsTerminal) rail() string {
	return t.style.faint(railGlyph)
}

// breakLine draws the blank line before a section, an item or a result. Right after a spinner the
// gap is already there, so no second one is drawn.
func (t *portalsTerminal) breakLine() {
	if t.spinnerGap {
		t.spinnerGap = false
		return
	}

	t.println(t.rail())
}

// message draws text on the rail. Every line of the text starts on the rail.
func (t *portalsTerminal) message(text string) {
	for _, line := range strings.Split(text, "\n") {
		t.println(t.rail() + "  " + line)
	}
}

// symbol draws text after a mark. The lines after the first start on the rail.
func (t *portalsTerminal) symbol(mark, text string) {
	for index, line := range strings.Split(text, "\n") {
		if index == 0 {
			t.println(mark + "  " + line)
			continue
		}

		t.println(t.rail() + "  " + line)
	}
}

// Banner implements portals.Reporter. It draws the title in the top border of a rounded box and the
// tenant inside, so a wrong tenant is visible from the start.
func (t *portalsTerminal) Banner(title, tenantDomain string) {
	if t.silent {
		return
	}

	// The width is the widest of the title and the content with their padding, rounded up to an even number.
	width := max(utf8.RuneCountInString(title)+2, utf8.RuneCountInString(tenantDomain)+4)
	width += width % 2

	titleSpace := width - utf8.RuneCountInString(title)
	contentSpace := width - utf8.RuneCountInString(tenantDomain)

	prefix := t.rail() + " "
	border := func(text string) string { return t.style.faint(text) }

	t.println(prefix +
		border("╭"+strings.Repeat("─", titleSpace/2)) +
		t.style.bold(title) +
		border(strings.Repeat("─", titleSpace-titleSpace/2)+"╮"))
	t.println(prefix +
		border("│") + strings.Repeat(" ", contentSpace/2) +
		t.style.cyan(tenantDomain) +
		strings.Repeat(" ", contentSpace-contentSpace/2) + border("│"))
	t.println(prefix + border("╰"+strings.Repeat("─", width)+"╯"))
}

// Section implements portals.Reporter.
func (t *portalsTerminal) Section(title string) {
	if t.silent {
		return
	}

	t.breakLine()
	t.symbol(t.style.green(stepGlyph), title)
}

// Item implements portals.Reporter.
func (t *portalsTerminal) Item(text, outcome string) {
	if t.silent {
		return
	}

	t.breakLine()
	t.message(withOutcome(text, outcome, t.style))
}

// Fixed implements portals.Reporter.
func (t *portalsTerminal) Fixed(text, summary string) {
	if t.silent {
		return
	}

	t.breakLine()
	t.message(withOutcome(t.style.green("✓")+" "+text, summary, t.style))
}

func withOutcome(text, outcome string, style portalsStyle) string {
	if outcome == "" {
		return text
	}

	return text + " " + style.faint(outcome)
}

// Result implements portals.Reporter.
func (t *portalsTerminal) Result(text string) {
	if t.silent {
		return
	}

	t.breakLine()
	t.symbol(t.style.green(successGlyph), text)
}

// Info implements portals.Reporter.
func (t *portalsTerminal) Info(text string) {
	if t.silent {
		return
	}

	t.symbol(t.style.blue(infoGlyph), text)
}

// Warn implements portals.Reporter.
func (t *portalsTerminal) Warn(text string) {
	if t.silent {
		return
	}

	t.symbol(t.style.yellow(warnGlyph), text)
}

// Error implements portals.Reporter.
func (t *portalsTerminal) Error(text string) {
	if t.silent {
		return
	}

	t.symbol(t.style.red(errorGlyph), text)
}

// Detail implements portals.Reporter.
func (t *portalsTerminal) Detail(text string) {
	if t.silent {
		return
	}

	t.message(t.style.faint(text))
}

// Reference implements portals.Reporter. Terminals that support links make the ID clickable, and
// the plain ID is shown where colors are off.
func (t *portalsTerminal) Reference(label, id, url string) {
	if t.silent {
		return
	}

	t.message(t.style.faint(label+":") + " " + t.style.link(url, t.style.cyan(id)))
}

// Ready implements portals.Reporter. The URL of the portal is in a note, closing the output of a
// successful setup.
func (t *portalsTerminal) Ready(url string) {
	if t.silent {
		return
	}

	const title = "Portal URL"

	t.breakLine()

	content := "Open portal: " + url
	width := max(utf8.RuneCountInString(content), utf8.RuneCountInString(title)) + 2
	blank := strings.Repeat(" ", width+2)

	t.println(t.rail())
	t.println(t.style.green(stepGlyph) + "  " + title + " " +
		t.style.faint(strings.Repeat("─", max(width-utf8.RuneCountInString(title)-1, 1))+"╮"))
	t.println(t.rail() + blank + t.rail())
	t.println(t.rail() + "  " + t.style.faint("Open portal:") + " " + t.style.link(url, t.style.cyan(url)) +
		strings.Repeat(" ", width-utf8.RuneCountInString(content)) + t.rail())
	t.println(t.rail() + blank + t.rail())
	t.println(t.style.faint("├" + strings.Repeat("─", width+2) + "╯"))
}

// Cancelled implements portals.Reporter.
func (t *portalsTerminal) Cancelled(message string) {
	if t.silent {
		return
	}

	t.println(t.style.faint(endGlyph) + "  " + t.style.red(message))
	t.println("")
}

// Table implements portals.Reporter. The widths follow the longest cell, and the first column is a
// symbol colored by what it is.
func (t *portalsTerminal) Table(headers []string, rows [][]string) {
	if t.silent {
		return
	}

	widths := make([]int, len(headers))
	for column, header := range headers {
		widths[column] = utf8.RuneCountInString(header)

		for _, row := range rows {
			widths[column] = max(widths[column], utf8.RuneCountInString(row[column]))
		}
	}

	format := func(cells []string, paint func(cell string, column int) string) string {
		padded := make([]string, 0, len(cells))

		for column, cell := range cells {
			cell += strings.Repeat(" ", widths[column]-utf8.RuneCountInString(cell))

			if paint != nil {
				cell = paint(cell, column)
			}

			padded = append(padded, cell)
		}

		return strings.Join(padded, "  ")
	}

	t.message(t.style.faint(format(headers, nil)))

	for _, row := range rows {
		paintSymbol := t.style.yellow
		if row[0] == "✖" {
			paintSymbol = t.style.red
		}

		t.message(format(row, func(cell string, column int) string {
			if column == 0 {
				return paintSymbol(cell)
			}

			return cell
		}))
	}
}

// spinnerFrames are the frames of the spinner.
var spinnerFrames = []string{"◒", "◐", "◓", "◑"}

// Task implements portals.Reporter. The spinner line is cleared when the work ends, so the caller
// prints the result in its place. Without a terminal there is no spinner, only the work.
func (t *portalsTerminal) Task(title string, work func() error) error {
	file, isFile := t.out.(*os.File)
	if t.silent || !t.animate || !isFile {
		return work()
	}

	// The spinner starts on a line of its own, and the rail line above it is the blank line that
	// comes before the next line.
	t.println(t.rail())

	running := spinner.New(spinnerFrames, 80*time.Millisecond, spinner.WithWriterFile(file))
	running.Suffix = "  " + title
	running.HideCursor = true

	if err := running.Color("cyan"); err != nil {
		return work()
	}

	running.Start()
	err := work()
	running.Stop()

	t.spinnerGap = true

	return err
}
