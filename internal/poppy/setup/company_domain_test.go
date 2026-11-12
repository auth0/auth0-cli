package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeCompanyDomain(t *testing.T) {
	tests := map[string]string{
		"shop.example":          "shop.example",
		"Shop.Example":          "shop.example",
		"https://shop.example":  "shop.example",
		"HTTPS://Shop.Example/": "shop.example",
	}

	for entry, want := range tests {
		t.Run(entry, func(t *testing.T) {
			assert.Equal(t, want, NormalizeCompanyDomain(entry))
		})
	}
}

func TestHasCompanyDomain(t *testing.T) {
	assert.True(t, HasCompanyDomain([]string{"a.example", "shop.example"}, "shop.example"))
	assert.True(t, HasCompanyDomain([]string{"https://shop.example"}, "shop.example"), "an origin written by an older build names the same host")
	assert.True(t, HasCompanyDomain([]string{"Shop.Example"}, "shop.example"))
	assert.False(t, HasCompanyDomain([]string{"https://shop.example/store"}, "shop.example"))
	assert.False(t, HasCompanyDomain(nil, "shop.example"))
}

func TestCompanyDomainsWith(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    []string
	}{
		{name: "empty", want: []string{"shop.example"}},
		{name: "appended", entries: []string{"a.example"}, want: []string{"a.example", "shop.example"}},
		{name: "origins become bare hosts", entries: []string{"https://a.example", "B.example"}, want: []string{"a.example", "b.example", "shop.example"}},
		{name: "repeats are dropped", entries: []string{"https://shop.example", "shop.example"}, want: []string{"shop.example"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, companyDomainsWith(test.entries, "shop.example"))
		})
	}
}
