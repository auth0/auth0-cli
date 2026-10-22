package proxy

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The limits on what a plugin can send in the request line.
const (
	maxPathBytes = 2048
	maxQuery     = 8 << 10
	maxSegments  = 18
)

// target is the part of a request that the proxy checked and forwards.
type target struct {
	// The path is decoded. It is both what the scope check looks at and what is
	// forwarded, so the two cannot differ.
	path string

	// The segments are those of the path after the Management API prefix.
	segments []string

	// The query is the raw query string, forwarded as it came.
	query string
}

// parseTarget checks the request target of a request, which is the raw text of
// the request line, and returns what the proxy forwards. A path that is not
// already canonical is rejected, never repaired: the proxy does not clean up
// dot segments, repeated slashes or escapes, because a repair is a second
// interpretation of the path that could differ from the one the server behind
// the proxy makes.
func parseTarget(requestURI string) (target, *rejection) {
	if !strings.HasPrefix(requestURI, "/") {
		return target{}, reject(http.StatusBadRequest, codeBadRequest,
			"The request target "+quoted(requestURI)+" must be a path that starts with /.")
	}

	rawPath, query, _ := strings.Cut(requestURI, "?")

	if len(rawPath) > maxPathBytes || len(query) > maxQuery {
		return target{}, reject(http.StatusRequestURITooLong, codeURITooLong, "The request target is too long.")
	}

	if !validQuery(query) {
		return target{}, reject(http.StatusBadRequest, codeBadRequest, "The query string "+quoted(query)+" has characters that are not allowed.")
	}

	path, problem := decodePath(rawPath)
	if problem != "" {
		return target{}, reject(http.StatusBadRequest, codeBadRequest, "The path "+quoted(rawPath)+" is not allowed: "+problem+".")
	}

	segments := strings.Split(path[1:], "/")
	if len(segments) > maxSegments {
		return target{}, reject(http.StatusBadRequest, codeBadRequest, "The path "+quoted(rawPath)+" has too many segments.")
	}

	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return target{}, reject(http.StatusBadRequest, codeBadRequest,
				"The path "+quoted(rawPath)+" is not allowed: it has an empty or dot segment, or a trailing slash.")
		}
	}

	if !strings.HasPrefix(path, managementPrefix) {
		return target{}, reject(http.StatusForbidden, codeForbiddenPath,
			"Plugins can only call the Management API, under "+managementPrefix+". The path "+quoted(safeText(path))+" is not allowed.")
	}

	// The prefix is the first two segments, and no segment is empty, so the rest
	// of the segments are the ones after it.
	return target{path: path, segments: segments[2:], query: query}, nil
}

// decodePath decodes the escapes of a raw path and returns the decoded path. It
// returns a short reason instead when the path is not canonical: an escape that
// hides a slash, a dot, a backslash, a null byte or a percent sign, an escape
// of a character that needs none, or a character that has no business in a
// path.
func decodePath(raw string) (string, string) {
	var decoded strings.Builder

	decoded.Grow(len(raw))

	for i := 0; i < len(raw); i++ {
		switch c := raw[i]; {
		case c == '#':
			return "", "it has a fragment"
		case c != '%':
			decoded.WriteByte(c)
		case i+2 >= len(raw):
			return "", "it has an escape that is cut short"
		default:
			value, err := strconv.ParseUint(raw[i+1:i+3], 16, 8)
			if err != nil {
				return "", "it has an escape that is not hexadecimal"
			}

			c = byte(value)
			i += 2

			switch {
			case c == '/' || c == '.' || c == '\\' || c == 0 || c == '%':
				return "", "it has an escaped slash, dot, backslash, null byte or percent sign"
			case isUnreserved(c):
				return "", "it escapes a character that needs no escape"
			}

			decoded.WriteByte(c)
		}
	}

	path := decoded.String()

	if !utf8.ValidString(path) {
		return "", "it is not valid UTF-8"
	}

	if strings.ContainsAny(path, ";\\") || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return "", "it has a semicolon, a backslash or a control character"
	}

	return path, ""
}

// isUnreserved reports whether c is a character that a URL never needs to
// escape: a letter, a digit, or one of - _ . ~.
func isUnreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~'
}

// validQuery reports whether a raw query string only has printable ASCII
// characters and no fragment marker, so that it can be forwarded as it is.
func validQuery(query string) bool {
	for i := 0; i < len(query); i++ {
		if c := query[i]; c <= ' ' || c >= 0x7f || c == '#' {
			return false
		}
	}

	return true
}
