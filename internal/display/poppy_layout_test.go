package display

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// setColors turns the colors of the ansi package on or off for one test.
func setColors(t *testing.T, on bool) {
	t.Helper()

	force, disable, override := ansi.ForceColors, ansi.DisableColors, ansi.EnvironmentOverrideColors
	ansi.ForceColors, ansi.EnvironmentOverrideColors = on, false
	ansi.Initialize(!on)
	t.Cleanup(func() {
		ansi.ForceColors, ansi.EnvironmentOverrideColors = force, override
		ansi.Initialize(disable)
	})
}

func TestPoppyLayoutOf(t *testing.T) {
	t.Run("a writer that is not a terminal is not wrapped", func(t *testing.T) {
		setColors(t, false)
		assert.Equal(t, PoppyLayout{}, PoppyLayoutOf(&bytes.Buffer{}))
	})

	t.Run("colors keep the Unicode symbols on a pipe", func(t *testing.T) {
		setColors(t, true)
		assert.Equal(t, PoppyLayout{Unicode: true}, PoppyLayoutOf(&bytes.Buffer{}))
	})

	t.Run("a file that is not a terminal is not wrapped", func(t *testing.T) {
		setColors(t, false)

		f, err := os.CreateTemp(t.TempDir(), "report")
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })

		assert.Equal(t, PoppyLayout{}, PoppyLayoutOf(f))
	})
}

func TestPoppyLineWidth(t *testing.T) {
	assert.Equal(t, poppyDefaultWidth, PoppyLayout{}.lineWidth())
	assert.Equal(t, 120, PoppyLayout{Width: 120}.lineWidth())
}

func TestPoppyWrap(t *testing.T) {
	words := strings.Repeat("word ", 30)

	tests := []struct {
		name   string
		layout PoppyLayout
		indent int
		want   []string
	}{
		{name: "no width keeps one line", layout: PoppyLayout{}, want: []string{strings.TrimSpace(words)}},
		{name: "wraps at the width less the indent", layout: PoppyLayout{Width: 20}, indent: 0, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.layout.wrap(words, test.indent)

			if test.want != nil {
				assert.Equal(t, test.want, got)

				return
			}

			require.Greater(t, len(got), 1)
			for _, line := range got {
				assert.LessOrEqual(t, utf8.RuneCountInString(line), 20)
			}
		})
	}

	assert.Equal(t, []string{"aaaa", "aaaa", "aa"}, wrapWords("aaaaaaaaaa", 4), "a word longer than a line is cut")
	assert.Equal(t, []string{""}, wrapWords("", 10), "there is always a line")
	assert.Equal(t, []string{"a b", "c"}, wrapWords("a   b c", 3))
	assert.Len(t, PoppyLayout{Width: 5}.wrap(strings.Repeat("x", 100), 0), 5, "the width never goes below the minimum of 20")
}

func TestPoppyHang(t *testing.T) {
	l := PoppyLayout{Width: 40, Unicode: true}

	got := l.hang("  ✓ API  ", "  X API  ", "alpha beta gamma delta epsilon zeta eta theta")

	assert.Equal(t, "  X API  alpha beta gamma delta epsilon\n         zeta eta theta\n", got)
	assert.Equal(t, "  ✓ API  alpha beta gamma delta epsilon\n", PoppyLayout{}.hang("  ✓ API  ", "  ✓ API  ", "alpha beta gamma delta epsilon"))
}

func TestPoppyMarks(t *testing.T) {
	setColors(t, false)

	tests := []struct {
		mark        poppyMark
		unicode     string
		ascii       string
		unicodeOnly bool
	}{
		{markDone, "✓", "ok", false},
		{markChanged, "~", "~", false},
		{markAdded, "+", "+", false},
		{markWarning, "!", "!", false},
		{markFailed, "✗", "x", false},
		{markSkipped, "-", "-", false},
	}

	for _, test := range tests {
		symbol, colored := PoppyLayout{Unicode: true}.mark(test.mark)
		assert.Equal(t, test.unicode, symbol)
		assert.Equal(t, test.unicode, colored, "no color codes when colors are off")

		symbol, _ = PoppyLayout{}.mark(test.mark)
		assert.Equal(t, test.ascii, symbol)
	}

	assert.Equal(t, "→", PoppyLayout{Unicode: true}.arrow())
	assert.Equal(t, "->", PoppyLayout{}.arrow())
	assert.Equal(t, "⚠", PoppyLayout{Unicode: true}.alert())
	assert.Equal(t, "!", PoppyLayout{}.alert())
	assert.Equal(t, 1, PoppyLayout{Unicode: true}.markWidth())
	assert.Equal(t, 2, PoppyLayout{}.markWidth())

	setColors(t, true)
	_, colored := PoppyLayout{Unicode: true}.mark(markFailed)
	assert.Equal(t, ansi.Red("✗"), colored)
}

func TestPoppyEntries(t *testing.T) {
	setColors(t, false)

	entries := []poppyEntry{
		{mark: markDone, label: "Agent app", value: "created"},
		{mark: markChanged, label: "DPoP", value: "required"},
		{mark: markWarning, value: "A note without a label."},
		{mark: markPlain, label: "name", value: "Meta Muse"},
		{mark: markPlain, value: "A line that is only indented."},
	}

	t.Run("unicode", func(t *testing.T) {
		assert.Equal(t, "  ✓ Agent app  created\n"+
			"  ~ DPoP       required\n"+
			"  ! A note without a label.\n"+
			"    name       Meta Muse\n"+
			"  A line that is only indented.\n",
			PoppyLayout{Unicode: true}.entries(entries))
	})

	t.Run("ascii", func(t *testing.T) {
		assert.Equal(t, "  ok Agent app  created\n"+
			"  ~  DPoP       required\n"+
			"  !  A note without a label.\n"+
			"     name       Meta Muse\n"+
			"  A line that is only indented.\n",
			PoppyLayout{}.entries(entries))
	})

	t.Run("a long value wraps in its column", func(t *testing.T) {
		out := PoppyLayout{Width: 40, Unicode: true}.entries([]poppyEntry{
			{mark: markDone, label: "Grant", value: "https://api.shop.example: orders:read checkout refunds wishlist"},
		})

		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, line)
		}

		assert.Contains(t, out, "\n          ", "the next lines sit under the value")
	})
}

func TestPoppyCanErase(t *testing.T) {
	t.Run("a writer that is not a terminal cannot", func(t *testing.T) {
		t.Setenv("TERM", "xterm")
		assert.False(t, PoppyCanErase(&bytes.Buffer{}))
	})

	t.Run("a dumb terminal cannot", func(t *testing.T) {
		t.Setenv("TERM", "dumb")
		assert.False(t, PoppyCanErase(&bytes.Buffer{}))
	})
}

func TestPoppyLineRows(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  int
	}{
		{name: "an empty line takes a row", line: "", width: 80, want: 1},
		{name: "a short line", line: "hello", width: 80, want: 1},
		{name: "a line of exactly the width", line: strings.Repeat("a", 80), width: 80, want: 1},
		{name: "one cell over the width", line: strings.Repeat("a", 81), width: 80, want: 2},
		{name: "several rows", line: strings.Repeat("a", 161), width: 80, want: 3},
		{name: "colors take no room", line: ansi.Bold(strings.Repeat("a", 80)), width: 80, want: 1},
		{name: "wide characters count as one cell each", line: strings.Repeat("é", 80), width: 80, want: 1},
		{name: "no width means no wrapping", line: strings.Repeat("a", 500), width: 0, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, poppyLineRows(test.line, test.width))
		})
	}
}

func TestPoppyBlock(t *testing.T) {
	long := strings.Repeat("a", 100)

	tests := []struct {
		name  string
		erase bool
		write []string
		rows  int
		// Want is what the writer got after the block was erased.
		want string
	}{
		{name: "one line", erase: true, write: []string{"title\n"}, rows: 1, want: "title\n\r\x1b[1A\x1b[J"},
		{name: "two lines", erase: true, write: []string{"title\n", "reason\n"}, rows: 2, want: "title\nreason\n\r\x1b[2A\x1b[J"},
		{name: "a line that wraps", erase: true, write: []string{long + "\n"}, rows: 2, want: long + "\n\r\x1b[2A\x1b[J"},
		{name: "a line written in parts", erase: true, write: []string{"ti", "tle\n"}, rows: 1, want: "title\n\r\x1b[1A\x1b[J"},
		{name: "several lines in one write", erase: true, write: []string{"a\nb\nc\n"}, rows: 3, want: "a\nb\nc\n\r\x1b[3A\x1b[J"},
		{name: "a blank line counts", erase: true, write: []string{"\n", "title\n"}, rows: 2, want: "\ntitle\n\r\x1b[2A\x1b[J"},
		{name: "nothing written, nothing erased", erase: true, write: nil, rows: 0, want: ""},
		{name: "a terminal that cannot erase keeps the lines", erase: false, write: []string{"title\n", "reason\n"}, rows: 2, want: "title\nreason\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer

			b := newPoppyBlock(&out, PoppyLayout{Width: 80}, test.erase)
			for _, part := range test.write {
				_, err := io.WriteString(b, part)
				require.NoError(t, err)
			}

			assert.Equal(t, test.rows, b.rows)
			require.NoError(t, b.erase())
			assert.Equal(t, test.want, out.String())
			assert.Zero(t, b.rows, "a block that was erased starts again")
		})
	}

	t.Run("rows that something else left are counted with Add", func(t *testing.T) {
		var out bytes.Buffer

		b := newPoppyBlock(&out, PoppyLayout{Width: 80}, true)
		_, _ = io.WriteString(b, "title\n")
		b.add(2)
		b.add(-5)

		require.NoError(t, b.erase())
		assert.True(t, strings.HasSuffix(out.String(), "\r\x1b[3A\x1b[J"), out.String())
	})

	t.Run("the next block counts its own rows", func(t *testing.T) {
		var out bytes.Buffer

		b := newPoppyBlock(&out, PoppyLayout{Width: 80}, true)
		_, _ = io.WriteString(b, "one\ntwo\n")
		require.NoError(t, b.erase())
		out.Reset()

		_, _ = io.WriteString(b, "three\n")
		require.NoError(t, b.erase())
		assert.Equal(t, "three\n\r\x1b[1A\x1b[J", out.String())
	})

	t.Run("an unfinished line is not counted", func(t *testing.T) {
		var out bytes.Buffer

		b := newPoppyBlock(&out, PoppyLayout{Width: 80}, true)
		_, _ = io.WriteString(b, "title\nprompt")

		assert.Equal(t, 1, b.rows)
	})
}

func TestPoppyHeader(t *testing.T) {
	setColors(t, false)

	assert.Equal(t, "Poppy setup"+strings.Repeat(" ", 80-len("Poppy setup")-len("tenant x"))+"tenant x\n",
		PoppyLayout{Width: 80}.header("Poppy setup", "tenant x"))
	assert.Equal(t, "A very long title tenant x\n", PoppyLayout{Width: 10}.header("A very long title", "tenant x"), "at least one space")
}
