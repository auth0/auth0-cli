package openapi

import (
	"regexp"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/getkin/kin-openapi/openapi3"
)

// patternMatchTimeout caps a single regexp2 match so a slow pattern can't hang the CLI.
const patternMatchTimeout = 100 * time.Millisecond

// unicodeEscape matches the ECMA-262 \uXXXX escape, which RE2 writes as \x{XXXX}.
var unicodeEscape = regexp.MustCompile(`\\u([0-9A-Fa-f]{4})`)

// compilePattern compiles a schema "pattern". Spec patterns use ECMA-262 syntax,
// and some use lookarounds that RE2 can't compile. RE2 is tried first because it
// matches in linear time; patterns it rejects fall back to regexp2.
func compilePattern(expr string) (openapi3.RegexMatcher, error) {
	if re, err := regexp.Compile(unicodeEscape.ReplaceAllString(expr, `\x{$1}`)); err == nil {
		return re, nil
	}

	re, err := regexp2.Compile(expr, regexp2.None)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = patternMatchTimeout
	return backtrackingMatcher{re: re}, nil
}

// backtrackingMatcher adapts regexp2 to kin-openapi's RegexMatcher.
type backtrackingMatcher struct {
	re *regexp2.Regexp
}

// MatchString reports whether s matches. A timed-out match counts as a match and
// leaves the check to the API.
func (m backtrackingMatcher) MatchString(s string) bool {
	ok, err := m.re.MatchString(s)
	return ok || err != nil
}
