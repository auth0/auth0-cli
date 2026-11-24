package portals

import (
	"regexp"
	"strings"
)

// SlugPattern is what the API accepts as the slug of a portal.
const SlugPattern = "^[a-z][a-z0-9-]{0,99}$"

var (
	slugRegexp        = regexp.MustCompile(SlugPattern)
	notSlugCharacters = regexp.MustCompile(`[^a-z0-9]+`)
	notALetterAtStart = regexp.MustCompile(`^[^a-z]+`)
)

const (
	maxSlugLength = 100
	// The slug of a portal whose name leaves nothing to make one from.
	fallbackSlug = "my-portal"
)

// IsValidSlug reports whether the API accepts the slug.
func IsValidSlug(slug string) bool {
	return slugRegexp.MatchString(slug)
}

// SlugFromName makes the default slug of a portal from its name, for example "Acme Admin" becomes
// "acme-admin".
func SlugFromName(name string) string {
	slug := notSlugCharacters.ReplaceAllString(strings.ToLower(name), "-")
	slug = strings.Trim(slug, "-")
	slug = notALetterAtStart.ReplaceAllString(slug, "")

	if len(slug) > maxSlugLength {
		slug = slug[:maxSlugLength]
	}

	slug = strings.TrimRight(slug, "-")
	if slug == "" {
		return fallbackSlug
	}

	return slug
}
