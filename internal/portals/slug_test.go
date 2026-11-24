package portals_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestIsValidSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slug string
		want bool
	}{
		{name: "lowercase with a hyphen", slug: "acme-admin", want: true},
		{name: "a single letter", slug: "a", want: true},
		{name: "the longest one", slug: "a" + strings.Repeat("b", 99), want: true},
		{name: "a space and capitals", slug: "My Portal"},
		{name: "a digit first", slug: "1portal"},
		{name: "a hyphen first", slug: "-portal"},
		{name: "an underscore", slug: "portal_one"},
		{name: "empty", slug: ""},
		{name: "longer than 100 characters", slug: "a" + strings.Repeat("b", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, portals.IsValidSlug(tt.slug))
		})
	}
}

func TestSlugFromName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "words become a hyphenated slug", in: "Acme Admin", want: "acme-admin"},
		{name: "punctuation and runs of it become one hyphen", in: "Acme & Co., Ltd!", want: "acme-co-ltd"},
		{name: "leading digits are dropped, because a slug starts with a letter", in: "123 portal", want: "portal"},
		{name: "hyphens at the ends are dropped", in: "--Acme--", want: "acme"},
		{name: "a name with nothing to make a slug from", in: "!!!", want: "my-portal"},
		{name: "an empty name", in: "", want: "my-portal"},
		{name: "accents are not letters of a slug", in: "Café Ñandú", want: "caf-and"},
		{name: "a long name is cut to 100 characters without ending in a hyphen", in: strings.Repeat("ab ", 60), want: strings.TrimRight(strings.Repeat("ab-", 60)[:100], "-")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := portals.SlugFromName(tt.in)

			assert.Equal(t, tt.want, got)
			assert.True(t, portals.IsValidSlug(got), "the slug of any name is valid")
		})
	}
}
