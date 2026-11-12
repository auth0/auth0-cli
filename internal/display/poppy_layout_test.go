package display

import (
	"bytes"
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

func TestLayoutOf(t *testing.T) {
	t.Run("a writer that is not a terminal is not wrapped", func(t *testing.T) {
		setColors(t, false)
		assert.Equal(t, Layout{}, LayoutOf(&bytes.Buffer{}))
	})

	t.Run("colors keep the Unicode symbols on a pipe", func(t *testing.T) {
		setColors(t, true)
		assert.Equal(t, Layout{Unicode: true}, LayoutOf(&bytes.Buffer{}))
	})

	t.Run("a file that is not a terminal is not wrapped", func(t *testing.T) {
		setColors(t, false)

		f, err := os.CreateTemp(t.TempDir(), "report")
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Close() })

		assert.Equal(t, Layout{}, LayoutOf(f))
	})
}

func TestPoppyLineWidth(t *testing.T) {
	assert.Equal(t, poppyDefaultWidth, Layout{}.lineWidth())
	assert.Equal(t, 120, Layout{Width: 120}.lineWidth())
}

func TestPoppyWrap(t *testing.T) {
	words := strings.Repeat("word ", 30)

	tests := []struct {
		name   string
		layout Layout
		indent int
		want   []string
	}{
		{name: "no width keeps one line", layout: Layout{}, want: []string{strings.TrimSpace(words)}},
		{name: "wraps at the width less the indent", layout: Layout{Width: 20}, indent: 0, want: nil},
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
	assert.Len(t, Layout{Width: 5}.wrap(strings.Repeat("x", 100), 0), 5, "the width never goes below the minimum of 20")
}

func TestPoppyHang(t *testing.T) {
	l := Layout{Width: 40, Unicode: true}

	got := l.hang("  ✓ API  ", "  X API  ", "alpha beta gamma delta epsilon zeta eta theta")

	assert.Equal(t, "  X API  alpha beta gamma delta epsilon\n         zeta eta theta\n", got)
	assert.Equal(t, "  ✓ API  alpha beta gamma delta epsilon\n", Layout{}.hang("  ✓ API  ", "  ✓ API  ", "alpha beta gamma delta epsilon"))
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
		symbol, colored := Layout{Unicode: true}.mark(test.mark)
		assert.Equal(t, test.unicode, symbol)
		assert.Equal(t, test.unicode, colored, "no color codes when colors are off")

		symbol, _ = Layout{}.mark(test.mark)
		assert.Equal(t, test.ascii, symbol)
	}

	assert.Equal(t, "→", Layout{Unicode: true}.arrow())
	assert.Equal(t, "->", Layout{}.arrow())
	assert.Equal(t, "⚠", Layout{Unicode: true}.alert())
	assert.Equal(t, "!", Layout{}.alert())
	assert.Equal(t, 1, Layout{Unicode: true}.markWidth())
	assert.Equal(t, 2, Layout{}.markWidth())

	setColors(t, true)
	_, colored := Layout{Unicode: true}.mark(markFailed)
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
			Layout{Unicode: true}.entries(entries))
	})

	t.Run("ascii", func(t *testing.T) {
		assert.Equal(t, "  ok Agent app  created\n"+
			"  ~  DPoP       required\n"+
			"  !  A note without a label.\n"+
			"     name       Meta Muse\n"+
			"  A line that is only indented.\n",
			Layout{}.entries(entries))
	})

	t.Run("a long value wraps in its column", func(t *testing.T) {
		out := Layout{Width: 40, Unicode: true}.entries([]poppyEntry{
			{mark: markDone, label: "Grant", value: "https://api.shop.example: orders:read checkout refunds wishlist"},
		})

		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), 40, line)
		}

		assert.Contains(t, out, "\n          ", "the next lines sit under the value")
	})
}

func TestPoppyCells(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "empty", in: "", want: 0},
		{name: "ascii", in: "hello", want: 5},
		{name: "a check mark", in: "✓", want: 1},
		{name: "an arrow", in: "→", want: 1},
		{name: "an accented letter", in: "é", want: 1},
		{name: "wide characters take two cells", in: "日本語", want: 6},
		{name: "fullwidth forms take two cells", in: "ＡＢ", want: 4},
		{name: "mixed", in: "ab 日本 → ok", want: 12},
		{name: "colors take none", in: ansi.Bold("日本") + " ok", want: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setColors(t, true)
			assert.Equal(t, test.want, poppyCells(test.in))
		})
	}
}

func TestPoppyWrapCells(t *testing.T) {
	l := Layout{Width: 20}

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "a sentence of wide characters is cut at the width",
			in:   "日本語の説明文です日本語の説明文です",
			want: []string{"日本語の説明文です日", "本語の説明文です"},
		},
		{
			name: "words of wide characters wrap on their gaps",
			in:   "日本語の説明 文です 日本語の説明 文です",
			want: []string{"日本語の説明 文です", "日本語の説明 文です"},
		},
		{
			name: "ascii still wraps by characters",
			in:   "aaaa bbbb cccc dddd eeee",
			want: []string{"aaaa bbbb cccc dddd", "eeee"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines := l.wrap(test.in, 0)
			assert.Equal(t, test.want, lines)

			for _, line := range lines {
				assert.LessOrEqual(t, poppyCells(line), 20, line)
			}
		})
	}
}

func TestPoppyHeader(t *testing.T) {
	setColors(t, false)

	assert.Equal(t, "Poppy setup"+strings.Repeat(" ", 80-len("Poppy setup")-len("tenant x"))+"tenant x\n",
		Layout{Width: 80}.header("Poppy setup", "tenant x"))
	assert.Equal(t, "A very long title tenant x\n", Layout{Width: 10}.header("A very long title", "tenant x"), "at least one space")
}
